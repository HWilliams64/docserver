package integration_tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings" // Added for strings.Contains
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require" // Added for assertions
)

const (
	serverBinaryPath = "./app_binary" // Relative to integration_tests directory
	testDbPath       = "./test_docs.json" // Relative to integration_tests directory
	testPort         = "8081"
	serverBaseURL    = "http://localhost:" + testPort
	testJwtSecret    = "a-very-secure-secret-for-testing-only" // Fixed secret for predictable tokens
	readinessTimeout = 15 * time.Second // Max time to wait for server start
	readinessPoll    = 200 * time.Millisecond // How often to check if server is ready
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second, // Sensible timeout for HTTP requests
}

// --- Test Main: Setup & Teardown ---

func TestMain(m *testing.M) {
	log.Println("INFO: Starting integration test setup...")

	// --- 1. Build the server binary ---
	log.Println("INFO: Building server binary...")
	buildCmd := exec.Command("go", "build", "-o", serverBinaryPath, "../main.go")
	buildCmd.Dir = "." // Ensure build happens within integration_tests dir
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		log.Fatalf("FATAL: Failed to build server binary: %v\nOutput:\n%s", err, string(buildOutput))
	}
	log.Printf("INFO: Server binary built successfully at %s", serverBinaryPath)

	// Ensure binary path is absolute or relative to current dir for exec.Command
	absBinaryPath, _ := filepath.Abs(serverBinaryPath)
	absDbPath, _ := filepath.Abs(testDbPath)

	// --- 2. Prepare environment and arguments for the server ---
	// Use environment variables for configuration
	env := append(os.Environ(),
		fmt.Sprintf("DB_FILE=%s", absDbPath),
		fmt.Sprintf("JWT_SECRET=%s", testJwtSecret),
		fmt.Sprintf("PORT=%s", testPort),
		"ADDRESS=0.0.0.0", // Ensure it listens externally if needed, localhost is fine too
		"SAVE_INTERVAL=100ms", // Save quickly during tests
		"ENABLE_BACKUP=false", // No need for backups during tests
	)

	// --- 3. Run the server binary as a background process ---
	log.Printf("INFO: Starting server process: %s -port %s (DB: %s)", absBinaryPath, testPort, absDbPath)
	serverCmd := exec.Command(absBinaryPath) // Command only needs the binary path
	serverCmd.Env = env                       // Pass configuration via environment
	serverCmd.Stdout = os.Stdout              // Pipe server output to test output
	serverCmd.Stderr = os.Stderr
	// Start the process in the background
	err = serverCmd.Start()
	if err != nil {
		log.Fatalf("FATAL: Failed to start server process: %v", err)
	}
	log.Printf("INFO: Server process started (PID: %d)", serverCmd.Process.Pid)

	// --- 4. Wait for the server to be ready ---
	log.Printf("INFO: Waiting for server to become ready at %s...", serverBaseURL)
	ready := waitForServerReady(serverBaseURL+"/swagger/index.html", readinessTimeout) // Use Swagger UI path as health check
	if !ready {
		// Attempt to kill the process if it didn't become ready
		_ = serverCmd.Process.Signal(syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		_ = serverCmd.Process.Kill()
		log.Fatalf("FATAL: Server did not become ready within %v", readinessTimeout)
	}
	log.Println("INFO: Server is ready!")

	// --- 5. Run the actual tests ---
	log.Println("INFO: Running test functions...")
	exitCode := m.Run()
	log.Printf("INFO: Test functions finished with exit code %d.", exitCode)

	// --- 6. Teardown: Stop the server process ---
	log.Println("INFO: Tearing down - stopping server process...")
	// Send SIGTERM first for graceful shutdown attempt
	err = serverCmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		log.Printf("WARN: Failed to send SIGTERM to server process: %v", err)
	} else {
		// Wait a short moment for graceful shutdown
		time.Sleep(500 * time.Millisecond)
	}
	// Ensure it's stopped, kill if necessary
	err = serverCmd.Process.Kill()
	if err != nil && !strings.Contains(err.Error(), "process already finished") { // Ignore error if already stopped
		log.Printf("WARN: Failed to kill server process: %v", err)
	} else {
		log.Println("INFO: Server process stopped.")
	}
	// Wait for the process to release resources (optional but good practice)
	_, _ = serverCmd.Process.Wait()

	// --- 7. Teardown: Clean up artifacts ---
	log.Println("INFO: Cleaning up test artifacts...")
	err = os.Remove(serverBinaryPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Failed to remove server binary '%s': %v", serverBinaryPath, err)
	}
	err = os.Remove(testDbPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Failed to remove test database file '%s': %v", testDbPath, err)
	}
	// Also remove the .key file if it was generated
	keyFilePath := "./docs.key" // Assuming it's created in the same dir as the binary runs
	err = os.Remove(keyFilePath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Failed to remove generated key file '%s': %v", keyFilePath, err)
	}

	log.Println("INFO: Integration test teardown complete.")
	os.Exit(exitCode) // Exit with the code from m.Run()
}

// --- Helper Functions ---

// waitForServerReady polls a URL until it gets a 200 OK or times out.
func waitForServerReady(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return true // Server is ready
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(readinessPoll) // Wait before next poll
	}
	return false // Timeout reached
}

// makeRequest is a generic helper to make HTTP requests and handle basic errors/decoding.
// It automatically handles JSON marshalling for the body if provided.
// It returns the response, the decoded body (if targetStruct is provided), and any error.
func makeRequest(t *testing.T, method, urlPath string, authToken string, body interface{}, targetStruct interface{}) (*http.Response, error) {
	t.Helper() // Mark this as a test helper

	fullURL := serverBaseURL + urlPath
	var reqBody io.Reader
	var jsonData []byte
	var err error

	if body != nil {
		jsonData, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body for %s %s: %w", method, urlPath, err)
		}
		reqBody = bytes.NewBuffer(jsonData)
		log.Printf("DEBUG: Request %s %s Body: %s", method, urlPath, string(jsonData))
	} else {
		log.Printf("DEBUG: Request %s %s", method, urlPath)
	}


	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s %s: %w", method, urlPath, err)
	}

	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request %s %s: %w", method, urlPath, err)
	}
	// Defer closing the body, but read it first if needed
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, fmt.Errorf("failed to read response body for %s %s: %w", method, urlPath, err)
	}
	log.Printf("DEBUG: Response %s %s Status: %s Body: %s", method, urlPath, resp.Status, string(respBodyBytes))


	// Attempt to decode if a target struct is provided
	if targetStruct != nil && len(respBodyBytes) > 0 {
		// Use a new reader for the bytes we already read
		err = json.Unmarshal(respBodyBytes, targetStruct)
		if err != nil {
			// Don't fail the whole request, but log/return the decode error maybe?
			// For now, just return the raw response and a specific decode error
			return resp, fmt.Errorf("failed to decode JSON response for %s %s into %T: %w. Body: %s", method, urlPath, targetStruct, err, string(respBodyBytes))
		}
	}

	// If we got here, the request was made, body read, and optionally decoded.
	// Return the original response object along with nil error (unless decode failed).
	// The caller should check resp.StatusCode.
	return resp, nil // Decode error handled above
}


// --- API Request/Response Structs (add more as needed) ---

type SignupRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"` // Corrected field
	LastName  string `json:"last_name"`  // Corrected field
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	// Profile information is not included in the login response based on logs
}

// ProfileResponse matches the structure returned by /profiles/me
type ProfileResponse struct {
	ID             string    `json:"id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Email          string    `json:"email"`
	CreationDate   time.Time `json:"creation_date"`
	LastModifiedDate time.Time `json:"last_modified_date"`
	Extra          any       `json:"extra,omitempty"`
}

type CreateDocRequest struct {
	Content interface{} `json:"content"`
}

type CreateDocResponse struct {
	ID             string      `json:"id"`
	OwnerID        string      `json:"owner_id"`
	Content        interface{} `json:"content"`
	CreationDate   time.Time   `json:"creation_date"`
	LastModifiedDate time.Time `json:"last_modified_date"`
}

type ShareRequest struct {
	SharedWith []string `json:"shared_with"` // List of profile IDs
}

type ErrorResponse struct {
	Error string `json:"error"`
}


// --- Test Functions ---

// TestSharingWorkflow implements the 9-step scenario.
func TestSharingWorkflow(t *testing.T) {
	t.Log("INFO: Starting TestSharingWorkflow...")
	assert := require.New(t) // Use testify require for automatic test failure on error

	// --- User Details ---
	userAEmail := "user.a." + fmt.Sprintf("%d", time.Now().UnixNano()) + "@example.com" // Unique email per run
	userAPassword := "passwordA123"
	userAFirstName := "UserA" // Split name
	userALastName := "Alpha"  // Split name
	userBEmail := "user.b." + fmt.Sprintf("%d", time.Now().UnixNano()) + "@example.com" // Unique email per run
	userBPassword := "passwordB456"
	userBFirstName := "UserB" // Split name
	userBLastName := "Bravo"  // Split name

	var tokenA, tokenB string
	var profileAID, profileBID string
	var docID string

	// --- Step 1: Sign up User A ---
	t.Log("Step 1: Signing up User A...")
	signupAReq := SignupRequest{Email: userAEmail, Password: userAPassword, FirstName: userAFirstName, LastName: userALastName} // Use correct fields
	resp, err := makeRequest(t, http.MethodPost, "/auth/signup", "", signupAReq, nil)
	assert.NoError(err, "Step 1: Signup A request failed")
	assert.Equal(http.StatusCreated, resp.StatusCode, "Step 1: Signup A expected status 201")

	// --- Step 2: Sign up User B ---
	t.Log("Step 2: Signing up User B...")
	signupBReq := SignupRequest{Email: userBEmail, Password: userBPassword, FirstName: userBFirstName, LastName: userBLastName} // Use correct fields
	resp, err = makeRequest(t, http.MethodPost, "/auth/signup", "", signupBReq, nil)
	assert.NoError(err, "Step 2: Signup B request failed")
	assert.Equal(http.StatusCreated, resp.StatusCode, "Step 2: Signup B expected status 201")

	// --- Step 3: Login User A ---
	t.Log("Step 3: Logging in User A...")
	loginAReq := LoginRequest{Email: userAEmail, Password: userAPassword}
	var loginAResp LoginResponse
	resp, err = makeRequest(t, http.MethodPost, "/auth/login", "", loginAReq, &loginAResp)
	assert.NoError(err, "Step 3: Login A request failed")
	assert.Equal(http.StatusOK, resp.StatusCode, "Step 3: Login A expected status 200")
	assert.NotEmpty(loginAResp.Token, "Step 3: Login A token should not be empty")
	tokenA = loginAResp.Token
	t.Logf("Step 3: User A logged in (Token: %s...)", tokenA[:min(10, len(tokenA))])

	// --- Step 3b: Get User A's Profile ID ---
	t.Log("Step 3b: Getting User A's profile...")
	var profileAResp ProfileResponse
	resp, err = makeRequest(t, http.MethodGet, "/profiles/me", tokenA, nil, &profileAResp)
	assert.NoError(err, "Step 3b: Get profile A request failed")
	assert.Equal(http.StatusOK, resp.StatusCode, "Step 3b: Get profile A expected status 200")
	assert.NotEmpty(profileAResp.ID, "Step 3b: Profile A ID should not be empty")
	assert.Equal(userAEmail, profileAResp.Email, "Step 3b: Profile A email mismatch")
	profileAID = profileAResp.ID
	t.Logf("Step 3b: User A Profile ID obtained: %s", profileAID)


	// --- Step 4: User A creates a document ---
	t.Log("Step 4: User A creating document...")
	createDocContent := map[string]interface{}{"title": "Shared Document", "version": 1.0}
	createDocReq := CreateDocRequest{Content: createDocContent}
	var createDocResp CreateDocResponse
	resp, err = makeRequest(t, http.MethodPost, "/documents", tokenA, createDocReq, &createDocResp)
	assert.NoError(err, "Step 4: Create document request failed")
	assert.Equal(http.StatusCreated, resp.StatusCode, "Step 4: Create document expected status 201")
	assert.NotEmpty(createDocResp.ID, "Step 4: Created document ID should not be empty")
	assert.Equal(profileAID, createDocResp.OwnerID, "Step 4: Document owner ID should match User A")
	// Use type assertion safely
	respContentMap, ok := createDocResp.Content.(map[string]interface{})
	assert.True(ok, "Step 4: Document content should be a map")
	assert.Equal(createDocContent["title"], respContentMap["title"], "Step 4: Document content title mismatch")
	docID = createDocResp.ID
	t.Logf("Step 4: Document created (ID: %s)", docID)

	// --- Step 5: User A shares the document with User B ---
	// We need User B's ID first. We'll get it after User B logs in (Step 6).
	// Deferring the actual share request until after Step 6b.


	// --- Step 5: User A shares the document with User B ---
	// --- Step 6: Login User B ---
	t.Log("Step 6: Logging in User B...")
	loginBReq := LoginRequest{Email: userBEmail, Password: userBPassword} // Define here
	var loginBResp LoginResponse
	resp, err = makeRequest(t, http.MethodPost, "/auth/login", "", loginBReq, &loginBResp) // Corrected path
	assert.NoError(err, "Step 6: Login B request failed")
	assert.Equal(http.StatusOK, resp.StatusCode, "Step 6: Login B expected status 200")
	assert.NotEmpty(loginBResp.Token, "Step 6: Login B token should not be empty")
	tokenB = loginBResp.Token
	t.Logf("Step 6: User B logged in (Token: %s...)", tokenB[:min(10, len(tokenB))])

	// --- Step 6b: Get User B's Profile ID ---
	t.Log("Step 6b: Getting User B's profile...")
	var profileBResp ProfileResponse
	resp, err = makeRequest(t, http.MethodGet, "/profiles/me", tokenB, nil, &profileBResp)
	assert.NoError(err, "Step 6b: Get profile B request failed")
	assert.Equal(http.StatusOK, resp.StatusCode, "Step 6b: Get profile B expected status 200")
	assert.NotEmpty(profileBResp.ID, "Step 6b: Profile B ID should not be empty")
	assert.Equal(userBEmail, profileBResp.Email, "Step 6b: Profile B email mismatch")
	profileBID = profileBResp.ID
	t.Logf("Step 6b: User B Profile ID obtained: %s", profileBID)

	// --- Now perform Step 5: User A shares the document with User B ---
	t.Log("Step 5: User A sharing document with User B...")
	shareReq := ShareRequest{SharedWith: []string{profileBID}} // Now we have profileBID
	shareURL := fmt.Sprintf("/documents/%s/shares", docID)
	resp, err = makeRequest(t, http.MethodPut, shareURL, tokenA, shareReq, nil) // Use tokenA
	assert.NoError(err, "Step 5: Share document request failed")
	// Accept 200, 201, or 204 for share update/creation
	assert.Contains([]int{http.StatusOK, http.StatusCreated, http.StatusNoContent}, resp.StatusCode, "Step 5: Share document expected status 200, 201, or 204")
	t.Logf("Step 5: Document %s shared with User B (%s)", docID, profileBID)

	// --- Step 6: Login User B (again, to get a fresh token if needed, or reuse from pre-step) ---
	t.Log("Step 6: Logging in User B...")
	// Step 6 login already performed above to get tokenB and profileBID

	// --- Step 7: User B accesses the document ---
	t.Log("Step 7: User B accessing shared document...")
	docURL := fmt.Sprintf("/documents/%s", docID)
	var getDocRespB CreateDocResponse
	resp, err = makeRequest(t, http.MethodGet, docURL, tokenB, nil, &getDocRespB)
	assert.NoError(err, "Step 7: Get document request failed for User B")
	assert.Equal(http.StatusOK, resp.StatusCode, "Step 7: Get document expected status 200 for User B")
	assert.Equal(docID, getDocRespB.ID, "Step 7: Document ID mismatch for User B")
	// Use type assertion safely
	respContentMapB, okB := getDocRespB.Content.(map[string]interface{})
	assert.True(okB, "Step 7: Document content should be a map for User B")
	assert.Equal(createDocContent["title"], respContentMapB["title"], "Step 7: Document content title mismatch for User B")
	t.Logf("Step 7: User B successfully accessed document %s", docID)

	// --- Step 8 (Revised): User A (owner) edits the document ---
	t.Log("Step 8 (Revised): User A editing document...")
	editDocContent := map[string]interface{}{"title": "Shared Document", "version": 2.0, "updated_by": "User A"} // Content updated by A
	editDocReq := CreateDocRequest{Content: editDocContent}
	var editDocResp CreateDocResponse
	resp, err = makeRequest(t, http.MethodPut, docURL, tokenA, editDocReq, &editDocResp) // Use User A's token (tokenA)
	assert.NoError(err, "Step 8: Edit document request failed for User A")
	assert.Equal(http.StatusOK, resp.StatusCode, "Step 8: Edit document expected status 200 for User A")
	assert.Equal(docID, editDocResp.ID, "Step 8: Edited document ID mismatch")
	// Use type assertion safely
	editRespContentMap, okEdit := editDocResp.Content.(map[string]interface{})
	assert.True(okEdit, "Step 8: Edited document content should be a map")
	assert.Equal(editDocContent["version"], editRespContentMap["version"], "Step 8: Edited document content version mismatch")
	assert.Equal(editDocContent["updated_by"], editRespContentMap["updated_by"], "Step 8: Edited document content updated_by mismatch")
	t.Logf("Step 8: User A successfully edited document %s", docID)

	// --- Step 9 (Revised): User B views the changes made by User A ---
	t.Log("Step 9 (Revised): User B viewing changes made by User A...")
	// Removed unused: var getDocRespA CreateDocResponse
	var getDocRespBAfterEdit CreateDocResponse
	resp, err = makeRequest(t, http.MethodGet, docURL, tokenB, nil, &getDocRespBAfterEdit) // Use User B's token (tokenB)
	assert.NoError(err, "Step 9: Get document request failed for User B")
	assert.Equal(http.StatusOK, resp.StatusCode, "Step 9: Get document expected status 200 for User B")
	assert.Equal(docID, getDocRespBAfterEdit.ID, "Step 9: Document ID mismatch for User B")
	// Use type assertion safely
	getRespContentMapB, okB := getDocRespBAfterEdit.Content.(map[string]interface{})
	assert.True(okB, "Step 9: Document content should be a map for User B")
	assert.Equal(editDocContent["version"], getRespContentMapB["version"], "Step 9: Document content version mismatch for User B (should show User A's changes)")
	assert.Equal(editDocContent["updated_by"], getRespContentMapB["updated_by"], "Step 9: Document content updated_by mismatch for User B (should show User A's changes)")
	t.Logf("Step 9: User B successfully viewed changes made by User A to document %s", docID)

	t.Log("INFO: TestSharingWorkflow completed successfully!")
}

// min helper for logging tokens safely
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}