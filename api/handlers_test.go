package api

import (
	"bytes"
	"docserver/config"
	"docserver/db"
	"docserver/utils"
	"encoding/json"
	"fmt" // Added
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings" // Added for case-insensitive comparison
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert" // Added
	"github.com/stretchr/testify/require"
)

// testJWTSecret is a fixed secret for generating tokens during tests.
const testJWTSecret = "test-integration-secret-key-needs-to-be-long-enough"

// setupTestServer initializes a Gin engine with routes and a temporary database for integration tests.
// It returns the configured router, the database instance, the test config, and a cleanup function.
func setupTestServer(t *testing.T) (*gin.Engine, *db.Database, *config.Config, func()) {
	gin.SetMode(gin.TestMode) // Set Gin to test mode

	// Create temp dir for DB file
	tempDir, err := os.MkdirTemp("", "docserver_api_test_")
	require.NoError(t, err, "Failed to create temp directory for test DB")

	// Create test config pointing to temp DB file and using fixed JWT secret
	cfg := &config.Config{
		DbFilePath:    filepath.Join(tempDir, "test_api_db.json"),
		SaveInterval:  10 * time.Millisecond, // Use a short interval for save tests if needed
		EnableBackup:  false,                 // Disable backup for simpler cleanup
		JwtSecret:     testJWTSecret,         // Use fixed secret for tests
		TokenLifetime: 1 * time.Hour,         // Standard token lifetime for tests
		BcryptCost:    4,                     // Minimum bcrypt cost for faster tests
		// ListenAddress and ListenPort are not used by httptest
	}

	// Create test database
	database, err := db.NewDatabase(cfg)
	require.NoError(t, err, "Failed to initialize test database")

	// Setup router exactly like in main.go
	router := gin.Default() // Use Default to include logger/recovery middleware like main
	router.RedirectTrailingSlash = false // Disable automatic redirect for trailing slashes

	// Public routes
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/signup", func(c *gin.Context) { SignupHandler(c, database, cfg) })
		authGroup.POST("/login", func(c *gin.Context) { LoginHandler(c, database, cfg) })
		authGroup.POST("/forgot-password", func(c *gin.Context) { ForgotPasswordHandler(c, database, cfg) })
		authGroup.POST("/reset-password", func(c *gin.Context) { ResetPasswordHandler(c, database, cfg) })
	}

	// Protected routes
	authMiddleware := utils.AuthMiddleware(cfg)

	profileGroup := router.Group("/profiles")
	profileGroup.Use(authMiddleware)
	{
		profileGroup.GET("/me", func(c *gin.Context) { GetProfileMeHandler(c, database, cfg) })
		profileGroup.PUT("/me", func(c *gin.Context) { UpdateProfileMeHandler(c, database, cfg) })
		profileGroup.DELETE("/me", func(c *gin.Context) { DeleteProfileMeHandler(c, database, cfg) })
		profileGroup.GET("", func(c *gin.Context) { SearchProfilesHandler(c, database, cfg) })
	}

	docGroup := router.Group("/documents")
	docGroup.Use(authMiddleware)
	{
		docGroup.POST("", func(c *gin.Context) { CreateDocumentHandler(c, database, cfg) })
		docGroup.GET("", func(c *gin.Context) { GetDocumentsHandler(c, database, cfg) })
		docGroup.GET("/:id", func(c *gin.Context) { GetDocumentByIDHandler(c, database, cfg) })
		docGroup.PUT("/:id", func(c *gin.Context) { UpdateDocumentHandler(c, database, cfg) })
		docGroup.DELETE("/:id", func(c *gin.Context) { DeleteDocumentHandler(c, database, cfg) })

		shareGroup := docGroup.Group("/:id/shares")
		{
			shareGroup.GET("", func(c *gin.Context) { GetSharersHandler(c, database, cfg) })
			shareGroup.PUT("", func(c *gin.Context) { SetSharersHandler(c, database, cfg) })
			shareGroup.PUT("/:profile_id", func(c *gin.Context) { AddSharerHandler(c, database, cfg) })
			shareGroup.DELETE("/:profile_id", func(c *gin.Context) { RemoveSharerHandler(c, database, cfg) })
		}
	}
	
	// Logout route
	router.POST("/auth/logout", authMiddleware, func(c *gin.Context) { LogoutHandler(c, database, cfg) })


	// Cleanup function to close the database and remove the temporary directory
	cleanup := func() {
		// Close the database first to ensure pending saves complete
		if err := database.Close(); err != nil {
			t.Logf("Warning: Error closing test database: %v", err)
		}
		// Now remove the temporary directory
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp directory %s: %v", tempDir, err)
		}
	}

	return router, database, cfg, cleanup
}

// performRequest executes an HTTP request against the test router.
// It automatically sets Content-Type to application/json for non-GET requests with a body.
// If token is provided, it adds the Authorization header.
func performRequest(router *gin.Engine, method, path string, body io.Reader, token string) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		panic(fmt.Sprintf("Failed to create request: %v", err)) // Panic in test helper is acceptable
	}

	// Set headers
	if body != nil && method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Perform the request
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// Helper to marshal data to JSON bytes buffer for request body
func marshalJSONBody(t *testing.T, data interface{}) *bytes.Buffer {
	bodyBytes, err := json.Marshal(data)
	require.NoError(t, err, "Failed to marshal JSON body for request")
	return bytes.NewBuffer(bodyBytes)
}
// createTestUserAndLogin signs up and logs in a new user for testing protected endpoints.
// Returns the user's ID, email, and auth token.
func createTestUserAndLogin(t *testing.T, router *gin.Engine, email, password, firstName, lastName string) (userID, userEmail, token string) {
	// Signup
	signupPayload := gin.H{
		"email":      email,
		"password":   password,
		"first_name": firstName,
		"last_name":  lastName,
	}
	signupRR := performRequest(router, "POST", "/auth/signup", marshalJSONBody(t, signupPayload), "")
	require.Equal(t, http.StatusCreated, signupRR.Code, "Signup should return 201 Created")
	var signupResp map[string]interface{}
	err := json.Unmarshal(signupRR.Body.Bytes(), &signupResp)
	require.NoError(t, err)
	userID = signupResp["id"].(string)
	userEmail = signupResp["email"].(string)
	require.NotEmpty(t, userID)

	// Login
	loginPayload := gin.H{"email": email, "password": password}
	loginRR := performRequest(router, "POST", "/auth/login", marshalJSONBody(t, loginPayload), "")
	require.Equal(t, http.StatusOK, loginRR.Code, "Login failed during test user creation")
	var loginResp map[string]string
	err = json.Unmarshal(loginRR.Body.Bytes(), &loginResp)
	require.NoError(t, err)
	token = loginResp["token"]
	require.NotEmpty(t, token)

	return userID, userEmail, token
}

// --- Authentication Endpoint Tests ---

func TestAuthEndpoints(t *testing.T) {
	router, database, _, cleanup := setupTestServer(t) // Use underscore for cfg if not directly needed in assertions here
	defer cleanup()

	var createdUserID string // To store ID from signup for later tests
	var userToken string     // To store token from login

	// --- Signup ---
	t.Run("Signup Success", func(t *testing.T) {
		signupPayload := gin.H{
			"email":      "test.signup@example.com",
			"password":   "password123",
			"first_name": "Test",
			"last_name":  "Signup",
		}
		rr := performRequest(router, "POST", "/auth/signup", marshalJSONBody(t, signupPayload), "")

		assert.Equal(t, http.StatusCreated, rr.Code) // Expect 201 Created

		// Check response body (should be the created profile, minus password hash)
		var responseBody map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &responseBody)
		require.NoError(t, err)
		assert.Equal(t, "test.signup@example.com", responseBody["email"])
		assert.Equal(t, "Test", responseBody["first_name"])
		assert.Equal(t, "Signup", responseBody["last_name"])
		assert.NotEmpty(t, responseBody["id"])
		assert.NotEmpty(t, responseBody["creation_date"])
		assert.NotEmpty(t, responseBody["last_modified_date"])
		assert.NotContains(t, responseBody, "password_hash", "Password hash should not be in signup response")

		createdUserID = responseBody["id"].(string) // Save for later

		// Check database state
		profile, found := database.GetProfileByEmail("test.signup@example.com")
		assert.True(t, found, "User should exist in database after signup")
		assert.Equal(t, createdUserID, profile.ID)
		assert.NotEmpty(t, profile.PasswordHash, "Password hash should be stored in database")
		// Verify the stored hash corresponds to the password
		assert.True(t, utils.CheckPasswordHash("password123", profile.PasswordHash), "Stored password hash is incorrect")
	})

	t.Run("Signup Duplicate Email", func(t *testing.T) {
		// Use the same email as the successful signup
		signupPayload := gin.H{
			"email":      "test.signup@example.com",
			"password":   "anotherpassword",
			"first_name": "Duplicate",
			"last_name":  "User",
		}
		rr := performRequest(router, "POST", "/auth/signup", marshalJSONBody(t, signupPayload), "")

		assert.Equal(t, http.StatusBadRequest, rr.Code) // Expect 400 Bad Request for duplicate email

		// Check error response
		var errorResponse map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse["error"], "email 'test.signup@example.com' already exists")
	})

	t.Run("Signup Missing Fields", func(t *testing.T) {
		// Example: Missing password
		signupPayload := gin.H{
			"email":      "missing.fields@example.com",
			"first_name": "Missing",
			"last_name":  "Fields",
		}
		rr := performRequest(router, "POST", "/auth/signup", marshalJSONBody(t, signupPayload), "")

		assert.Equal(t, http.StatusBadRequest, rr.Code) // Expect 400 for validation errors

		// Check error response (Gin's binding error format might vary slightly)
		var errorResponse map[string]interface{} // More general type for binding errors
		err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, rr.Body.String(), "Password", "Error message should mention missing Password field") // Simple string check
	})


	// --- Login ---
	t.Run("Login Success", func(t *testing.T) {
		require.NotEmpty(t, createdUserID, "Cannot run login test without successful signup") // Ensure signup ran first

		loginPayload := gin.H{
			"email":    "test.signup@example.com",
			"password": "password123", // Correct password from signup
		}
		rr := performRequest(router, "POST", "/auth/login", marshalJSONBody(t, loginPayload), "")

		assert.Equal(t, http.StatusOK, rr.Code)

		// Check response body for token
		var responseBody map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &responseBody)
		require.NoError(t, err)
		assert.NotEmpty(t, responseBody["token"], "Response should contain a JWT token")

		userToken = responseBody["token"] // Save for later tests
	})

	t.Run("Login Invalid Email", func(t *testing.T) {
		loginPayload := gin.H{
			"email":    "nonexistent@example.com",
			"password": "password123",
		}
		rr := performRequest(router, "POST", "/auth/login", marshalJSONBody(t, loginPayload), "")

		assert.Equal(t, http.StatusUnauthorized, rr.Code) // Expect 401 Unauthorized

		// Check error response
		var errorResponse map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, strings.ToLower(errorResponse["error"]), "invalid email or password", "Error message should contain 'invalid email or password' (case-insensitive)")
	})

	t.Run("Login Invalid Password", func(t *testing.T) {
		require.NotEmpty(t, createdUserID, "Cannot run login test without successful signup")

		loginPayload := gin.H{
			"email":    "test.signup@example.com",
			"password": "wrongpassword", // Incorrect password
		}
		rr := performRequest(router, "POST", "/auth/login", marshalJSONBody(t, loginPayload), "")

		assert.Equal(t, http.StatusUnauthorized, rr.Code) // Expect 401 Unauthorized

		// Check error response
		var errorResponse map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse["error"], "Invalid email or password") // Match actual casing
	})
	t.Run("Login Invalid JSON", func(t *testing.T) {
		// Send malformed JSON
		invalidJSON := `{"email": "test@example.com", "password": "password123"` // Missing closing brace
		rr := performRequest(router, "POST", "/auth/login", bytes.NewBufferString(invalidJSON), "")

		assert.Equal(t, http.StatusBadRequest, rr.Code) // Expect 400 Bad Request

		// Check error response
		var errorResponse map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse["error"], "Invalid request body", "Error message should indicate invalid request body")
	})

	
	// --- Logout ---
	t.Run("Logout Success", func(t *testing.T) {
		require.NotEmpty(t, userToken, "Cannot run logout test without successful login") // Ensure login ran first

		rr := performRequest(router, "POST", "/auth/logout", nil, userToken) // No body needed

		assert.Equal(t, http.StatusNoContent, rr.Code) // Expect 204 No Content
		assert.Empty(t, rr.Body.String(), "Logout response body should be empty")
	})

	t.Run("Logout No Token", func(t *testing.T) {
		rr := performRequest(router, "POST", "/auth/logout", nil, "") // No token provided

		assert.Equal(t, http.StatusUnauthorized, rr.Code) // Expect 401 Unauthorized (due to middleware)
	})

}
// --- Profile Endpoint Tests ---

func TestProfileEndpoints(t *testing.T) {
	router, database, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a test user
	userID, userEmail, token := createTestUserAndLogin(t, router, "profile.user@example.com", "profPass", "Profile", "User")
	require.NotEmpty(t, userID)
	require.NotEmpty(t, token)

	// Create another user for search tests
	_, _, token2 := createTestUserAndLogin(t, router, "search.user@example.com", "searchPass", "Search", "Person")
	require.NotEmpty(t, token2)


	// --- /profiles/me ---
	t.Run("Get Me Success", func(t *testing.T) {
		rr := performRequest(router, "GET", "/profiles/me", nil, token)
		assert.Equal(t, http.StatusOK, rr.Code)

		var profileResp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &profileResp)
		require.NoError(t, err)
		assert.Equal(t, userID, profileResp["id"])
		assert.Equal(t, userEmail, profileResp["email"])
		assert.Equal(t, "Profile", profileResp["first_name"])
		assert.Equal(t, "User", profileResp["last_name"])
		assert.NotContains(t, profileResp, "password_hash")
	})

	t.Run("Get Me No Token", func(t *testing.T) {
		rr := performRequest(router, "GET", "/profiles/me", nil, "") // No token
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Update Me Success", func(t *testing.T) {
		updatePayload := gin.H{
			"first_name": "UpdatedFirst",
			"last_name":  "UpdatedLast",
			"extra":      gin.H{"setting": "value"},
			// Email and password cannot be updated via this endpoint
		}
		rr := performRequest(router, "PUT", "/profiles/me", marshalJSONBody(t, updatePayload), token)
		assert.Equal(t, http.StatusOK, rr.Code)

		// Check response
		var profileResp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &profileResp)
		require.NoError(t, err)
		assert.Equal(t, userID, profileResp["id"])
		assert.Equal(t, "UpdatedFirst", profileResp["first_name"])
		assert.Equal(t, "UpdatedLast", profileResp["last_name"])
		assert.Equal(t, userEmail, profileResp["email"]) // Email should not change
		assert.NotNil(t, profileResp["extra"])
		extraMap, ok := profileResp["extra"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "value", extraMap["setting"])

		// Check database
		profile, found := database.GetProfileByID(userID)
		require.True(t, found)
		assert.Equal(t, "UpdatedFirst", profile.FirstName)
		assert.Equal(t, "UpdatedLast", profile.LastName)
		assert.NotEqual(t, profile.CreationDate, profile.LastModifiedDate) // Modified date should update
	})

	t.Run("Update Me Invalid Field", func(t *testing.T) {
		// Attempt to update email (should be ignored or cause error depending on handler strictness)
		updatePayload := gin.H{
			"email": "new.email@example.com", // Try to change email
			"first_name": "ShouldNotUpdate",
		}
		rr := performRequest(router, "PUT", "/profiles/me", marshalJSONBody(t, updatePayload), token)
		// Expecting 400 Bad Request because LastName is missing (required field)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		// Don't check response body for profile data on validation error
	})


	// --- /profiles (Search) ---
	t.Run("Search Profiles Success No Params", func(t *testing.T) {
		rr := performRequest(router, "GET", "/profiles", nil, token)
		assert.Equal(t, http.StatusOK, rr.Code)

		var searchResp struct { // Define struct for expected response format
			Data []map[string]interface{} `json:"data"`
			Total int `json:"total"`
			Page int `json:"page"`
			Limit int `json:"limit"`
		}
		err := json.Unmarshal(rr.Body.Bytes(), &searchResp)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, searchResp.Total, 2, "Should find at least 2 profiles") // The two created users
		assert.Equal(t, 1, searchResp.Page)
		assert.Contains(t, rr.Body.String(), `"profile.user@example.com"`) // Check if results contain expected data
		assert.Contains(t, rr.Body.String(), `"search.user@example.com"`)
	})

	t.Run("Search Profiles Success With Params", func(t *testing.T) {
		// Search for the second user by email fragment
		rr := performRequest(router, "GET", "/profiles?email=search.user", nil, token)
		assert.Equal(t, http.StatusOK, rr.Code)

		var searchResp struct {
			Data []map[string]interface{} `json:"data"`
			Total int `json:"total"`
		}
		err := json.Unmarshal(rr.Body.Bytes(), &searchResp)
		require.NoError(t, err)

		assert.Equal(t, 1, searchResp.Total, "Should find exactly 1 profile matching 'search.user'")
		require.Len(t, searchResp.Data, 1)
		assert.Equal(t, "search.user@example.com", searchResp.Data[0]["email"])
		assert.Equal(t, "Search", searchResp.Data[0]["first_name"])
	})

	t.Run("Search Profiles Pagination", func(t *testing.T) {
		// Assuming default limit is less than total users if many were created
		rr1 := performRequest(router, "GET", "/profiles?limit=1&page=1", nil, token)
		assert.Equal(t, http.StatusOK, rr1.Code)
		var resp1 struct { Data []map[string]interface{}; Total int; Page int; Limit int }
		err1 := json.Unmarshal(rr1.Body.Bytes(), &resp1)
		require.NoError(t, err1)
		assert.Len(t, resp1.Data, 1)
		assert.Equal(t, 1, resp1.Page)
		assert.Equal(t, 1, resp1.Limit)
		assert.GreaterOrEqual(t, resp1.Total, 2)
		firstUserID := resp1.Data[0]["id"]

		rr2 := performRequest(router, "GET", "/profiles?limit=1&page=2", nil, token)
		assert.Equal(t, http.StatusOK, rr2.Code)
		var resp2 struct { Data []map[string]interface{}; Total int; Page int; Limit int }
		err2 := json.Unmarshal(rr2.Body.Bytes(), &resp2)
		require.NoError(t, err2)
		assert.Len(t, resp2.Data, 1)
		assert.Equal(t, 2, resp2.Page)
		assert.Equal(t, 1, resp2.Limit)
		assert.GreaterOrEqual(t, resp2.Total, 2)
		secondUserID := resp2.Data[0]["id"]

		assert.NotEqual(t, firstUserID, secondUserID, "User IDs on page 1 and 2 should be different")
	})
	t.Run("Search Profiles Invalid Page Param", func(t *testing.T) {
		rr := performRequest(router, "GET", "/profiles?page=abc", nil, token2) // Use token2 as token1's user is deleted
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid 'page' or 'limit'")
	})

	t.Run("Search Profiles Invalid Limit Param", func(t *testing.T) {
		rr := performRequest(router, "GET", "/profiles?limit=xyz", nil, token2)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid 'page' or 'limit'")
	})

	t.Run("Search Profiles Page Zero", func(t *testing.T) {
		rr := performRequest(router, "GET", "/profiles?page=0", nil, token2)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid 'page' or 'limit'")
	})

	t.Run("Search Profiles Limit Over Max", func(t *testing.T) {
		rr := performRequest(router, "GET", "/profiles?limit=101", nil, token2)
		assert.Equal(t, http.StatusOK, rr.Code) // Should succeed, but limit capped
		var searchResp SearchProfilesResponse // Use the defined struct
		err := json.Unmarshal(rr.Body.Bytes(), &searchResp)
		require.NoError(t, err)
		assert.Equal(t, 100, searchResp.Limit, "Limit should be capped at 100")
		assert.GreaterOrEqual(t, searchResp.Total, 1, "Should find at least the remaining user") // Only user2 remains
	})

	t.Run("Search Profiles Page Out Of Bounds", func(t *testing.T) {
		// Assuming only 1 user (user2) remains after user1 deletion
		rr := performRequest(router, "GET", "/profiles?page=2&limit=10", nil, token2)
		assert.Equal(t, http.StatusOK, rr.Code) // Should return OK but empty data
		var searchResp SearchProfilesResponse
		err := json.Unmarshal(rr.Body.Bytes(), &searchResp)
		require.NoError(t, err)
		assert.Empty(t, searchResp.Data, "Data should be empty for out-of-bounds page")
		assert.Equal(t, 2, searchResp.Total, "Total should reflect the actual number of users (both exist at this point)") // Both users exist before deletion test
		assert.Equal(t, 2, searchResp.Page)
		assert.Equal(t, 10, searchResp.Limit)
	})


	// --- Delete Me ---
	// Run delete last as it removes the user
	t.Run("Delete Me Success", func(t *testing.T) {
		rr := performRequest(router, "DELETE", "/profiles/me", nil, token)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		// Verify user is deleted from DB
		_, found := database.GetProfileByID(userID)
		assert.False(t, found, "User should be deleted from database")

		// Try to get user again with same token (should fail)
		rrGet := performRequest(router, "GET", "/profiles/me", nil, token)
		assert.Equal(t, http.StatusNotFound, rrGet.Code, "User associated with token should not be found after deletion")
	})

	t.Run("Delete Me Not Found", func(t *testing.T) {
		// User 'profile.user@example.com' (associated with 'userID' and 'token')
		// was already deleted in the "Delete Me Success" test run just before this one.
		// Attempting to delete again using the same token should trigger the "not found" path
		// within the handler because database.DeleteProfile(userID) will return a "not found" error.
		rr := performRequest(router, "DELETE", "/profiles/me", nil, token) // Use the original token
		assert.Equal(t, http.StatusNotFound, rr.Code, "Deleting an already deleted user should return 404 Not Found")

		// Verify user is still not found in DB (using the original userID)
		_, found := database.GetProfileByID(userID)
		assert.False(t, found, "User should remain deleted from database")
	})
	t.Run("Update Me After Deletion", func(t *testing.T) {
		// User 'profile.user@example.com' was deleted in "Delete Me Success"
		// Attempt to update using the old token
		updatePayload := gin.H{
			"first_name": "Deleted",
			"last_name":  "UserUpdate",
		}
		rr := performRequest(router, "PUT", "/profiles/me", marshalJSONBody(t, updatePayload), token) // Use the original token
		assert.Equal(t, http.StatusNotFound, rr.Code, "Updating a deleted user's profile should return 404 Not Found")
	})

} // Closing brace for TestProfileEndpoints


// --- Document Endpoint Tests ---

func TestDocumentEndpoints(t *testing.T) {
	router, database, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create test users
	userID1, _, token1 := createTestUserAndLogin(t, router, "doc.user1@example.com", "docPass1", "Doc", "User1")
	_, _, token2 := createTestUserAndLogin(t, router, "doc.user2@example.com", "docPass2", "Doc", "User2") // Use blank identifier for userID2

	var createdDocID string // Store ID of doc created by user1

	// --- POST /documents ---
	t.Run("Create Document Success", func(t *testing.T) {
		docPayload := gin.H{
			"content": gin.H{"title": "My First Doc", "body": "Hello world"},
		}
		rr := performRequest(router, "POST", "/documents", marshalJSONBody(t, docPayload), token1)
		assert.Equal(t, http.StatusCreated, rr.Code, "Document creation should return 201 Created")

		var docResp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &docResp)
		require.NoError(t, err)
		assert.NotEmpty(t, docResp["id"], "Document ID should be in response")
		assert.Equal(t, userID1, docResp["owner_id"], "Owner ID should match user 1")
		assert.NotEmpty(t, docResp["creation_date"])
		assert.NotEmpty(t, docResp["last_modified_date"])
		contentMap, ok := docResp["content"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "My First Doc", contentMap["title"])

		createdDocID = docResp["id"].(string) // Save for later tests

		// Check database
		doc, found := database.GetDocumentByID(createdDocID)
		require.True(t, found, "Document not found in database")
		assert.Equal(t, userID1, doc.OwnerID)
	})

	t.Run("Create Document No Auth", func(t *testing.T) {
		docPayload := gin.H{"content": "No auth content"}
		rr := performRequest(router, "POST", "/documents", marshalJSONBody(t, docPayload), "") // No token
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Create Document Missing Content", func(t *testing.T) {
		// Send a request body without the required "content" field
		docPayload := gin.H{
			"other_field": "some value",
		}
		rr := performRequest(router, "POST", "/documents", marshalJSONBody(t, docPayload), token1)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Creating document without 'content' field should return 400 Bad Request")
	})

	// --- GET /documents ---
	t.Run("Get Documents Initial", func(t *testing.T) {
		// User 1 should have one document created above
		rr := performRequest(router, "GET", "/documents", nil, token1)
		assert.Equal(t, http.StatusOK, rr.Code)
		var listResp struct{ Data []map[string]interface{}; Total int }
		err := json.Unmarshal(rr.Body.Bytes(), &listResp)
		require.NoError(t, err)
		assert.Equal(t, 1, listResp.Total)
		require.Len(t, listResp.Data, 1)
		assert.Equal(t, createdDocID, listResp.Data[0]["id"])

		// User 2 should have zero documents initially
		rr2 := performRequest(router, "GET", "/documents", nil, token2)
		assert.Equal(t, http.StatusOK, rr2.Code)
		var listResp2 struct{ Data []map[string]interface{}; Total int }
		err2 := json.Unmarshal(rr2.Body.Bytes(), &listResp2)
		require.NoError(t, err2)
		assert.Equal(t, 0, listResp2.Total)
		assert.Empty(t, listResp2.Data)
	})

	t.Run("Get Documents Invalid Page Param", func(t *testing.T) {
		rr := performRequest(router, "GET", "/documents?page=abc", nil, token1)
		// Explicitly check the response code and body
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Expected 400 Bad Request for invalid page param")
		assert.Contains(t, rr.Body.String(), "Invalid 'page' or 'limit'", "Error message mismatch for invalid page param")
	})

	t.Run("Get Documents Invalid Limit Param", func(t *testing.T) {
		rr := performRequest(router, "GET", "/documents?limit=xyz", nil, token1)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid 'page' or 'limit'")
	})

	t.Run("Get Documents Page Zero", func(t *testing.T) {
		rr := performRequest(router, "GET", "/documents?page=0", nil, token1)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid 'page' or 'limit'")
	})

	t.Run("Get Documents Invalid Scope Param", func(t *testing.T) {
		// Assuming QueryDocuments returns an error containing "invalid scope value"
		rr := performRequest(router, "GET", "/documents?scope=invalidscope", nil, token1)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid scope value")
	})

	t.Run("Get Documents Invalid SortBy Param", func(t *testing.T) {
		rr := performRequest(router, "GET", "/documents?sort_by=invalidfield", nil, token1)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid sort_by value")
	})

	t.Run("Get Documents Invalid Order Param", func(t *testing.T) {
		rr := performRequest(router, "GET", "/documents?order=sideways", nil, token1)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid order value")
	})

	// --- GET /documents/{id} ---
	t.Run("Get Document By ID Success", func(t *testing.T) {
		require.NotEmpty(t, createdDocID, "Cannot run test without created document ID")
		rr := performRequest(router, "GET", "/documents/"+createdDocID, nil, token1)
		assert.Equal(t, http.StatusOK, rr.Code)

		var docResp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &docResp)
		require.NoError(t, err)
		assert.Equal(t, createdDocID, docResp["id"])
		assert.Equal(t, userID1, docResp["owner_id"])
		contentMap, ok := docResp["content"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "My First Doc", contentMap["title"])
	})

	t.Run("Get Document By ID Not Found", func(t *testing.T) {
		rr := performRequest(router, "GET", "/documents/non-existent-doc-id", nil, token1)
		// Expect 404 Not Found, as the handler checks db.GetDocumentByID
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Get Document By ID Not Authorized", func(t *testing.T) {
		require.NotEmpty(t, createdDocID, "Cannot run test without created document ID")
		// User 2 tries to get User 1's document
		rr := performRequest(router, "GET", "/documents/"+createdDocID, nil, token2)
		// Plan implies 404 if not owned or shared. Handler logic determines actual code.
		// Let's assume 404 based on typical REST patterns where unauthorized access to a specific resource ID often returns 404.
		assert.Equal(t, http.StatusForbidden, rr.Code) // Expect Forbidden because user2 doesn't have access
	})


	// --- PUT /documents/{id} ---
	t.Run("Update Document Success", func(t *testing.T) {
		require.NotEmpty(t, createdDocID, "Cannot run test without created document ID")
		updatePayload := gin.H{
			"content": gin.H{"title": "Updated Title", "body": "New body content"},
		}
		rr := performRequest(router, "PUT", "/documents/"+createdDocID, marshalJSONBody(t, updatePayload), token1)
		assert.Equal(t, http.StatusOK, rr.Code)

		var docResp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &docResp)
		require.NoError(t, err)
		assert.Equal(t, createdDocID, docResp["id"])
		contentMap, ok := docResp["content"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Updated Title", contentMap["title"])
		assert.Equal(t, "New body content", contentMap["body"])

		// Check database
		doc, found := database.GetDocumentByID(createdDocID)
		require.True(t, found)
		updatedContentMap, ok := doc.Content.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Updated Title", updatedContentMap["title"])
		assert.NotEqual(t, doc.CreationDate, doc.LastModifiedDate) // Modified date should update
	})

	t.Run("Update Document Not Found", func(t *testing.T) {
		updatePayload := gin.H{"content": "update non-existent"}
		rr := performRequest(router, "PUT", "/documents/non-existent-doc-id", marshalJSONBody(t, updatePayload), token1)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Update Document Not Authorized", func(t *testing.T) {
		require.NotEmpty(t, createdDocID, "Cannot run test without created document ID")
		updatePayload := gin.H{"content": "unauthorized update attempt"}
		// User 2 tries to update User 1's document
		rr := performRequest(router, "PUT", "/documents/"+createdDocID, marshalJSONBody(t, updatePayload), token2)
		// Expect 403 Forbidden or 404 Not Found depending on handler logic
		assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, rr.Code)
	})

	t.Run("Update Document Missing Content", func(t *testing.T) {
		require.NotEmpty(t, createdDocID, "Cannot run test without created document ID")
		updatePayload := gin.H{
			// Missing "content" field
			"other_field": "some value",
		}
		rr := performRequest(router, "PUT", "/documents/"+createdDocID, marshalJSONBody(t, updatePayload), token1)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Updating document without 'content' field should return 400 Bad Request")
	})
	t.Run("Update Document Missing ID", func(t *testing.T) {
		updatePayload := gin.H{"content": "update with missing id"}
		// Request path is missing the document ID
		rr := performRequest(router, "PUT", "/documents/", marshalJSONBody(t, updatePayload), token1)
		// Gin's routing might return 404 or 405 if no route matches /documents/ with PUT.
		// Let's check the handler logic again - it expects /:id, so a missing ID might not even reach the handler.
		// However, if a route like PUT /documents was defined, the handler's check `docID == ""` would trigger.
		// Assuming the route matches and the handler is reached:
		// assert.Equal(t, http.StatusBadRequest, rr.Code, "Updating document without ID in path should return 400 Bad Request")
		// More likely, Gin returns 404/405 if the route doesn't match. Let's test for that.
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rr.Code, "Requesting PUT /documents/ should result in 404 or 405")
	})


	// --- DELETE /documents/{id} ---
	t.Run("Delete Document Not Authorized", func(t *testing.T) {
		require.NotEmpty(t, createdDocID, "Cannot run test without created document ID")
		// User 2 tries to delete User 1's document
		rr := performRequest(router, "DELETE", "/documents/"+createdDocID, nil, token2)
		// Expect 403 Forbidden because the user exists but doesn't own the document
		assert.Equal(t, http.StatusForbidden, rr.Code)

		// Ensure doc still exists
		_, found := database.GetDocumentByID(createdDocID)
		assert.True(t, found, "Document should still exist after unauthorized delete attempt")
	})

	t.Run("Delete Document Success", func(t *testing.T) {
		require.NotEmpty(t, createdDocID, "Cannot run test without created document ID")
		// User 1 deletes their own document
		rr := performRequest(router, "DELETE", "/documents/"+createdDocID, nil, token1)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		// Verify document is deleted from DB
		_, found := database.GetDocumentByID(createdDocID)
		assert.False(t, found, "Document should be deleted from database")

		// Try to get the deleted document (should be 404)
		rrGet := performRequest(router, "GET", "/documents/"+createdDocID, nil, token1)
		assert.Equal(t, http.StatusNotFound, rrGet.Code)
	})

	t.Run("Delete Document Not Found", func(t *testing.T) {
		// Try deleting the already deleted doc ID or a non-existent one
		rr := performRequest(router, "DELETE", "/documents/"+createdDocID, nil, token1)
		assert.Equal(t, http.StatusNoContent, rr.Code, "Deleting a non-existent doc should be idempotent (204)")

		rr2 := performRequest(router, "DELETE", "/documents/completely-non-existent", nil, token1)
	assert.Equal(t, http.StatusNoContent, rr2.Code, "Deleting a non-existent doc again should be idempotent (204)")
})

t.Run("Delete Document No Auth", func(t *testing.T) {
	// Use the ID created earlier, even though it might be deleted now.
	// The point is to test the auth middleware.
	targetDocID := createdDocID
	if targetDocID == "" {
		targetDocID = "any-doc-id" // Fallback if creation failed
	}
	rr := performRequest(router, "DELETE", "/documents/"+targetDocID, nil, "") // No token
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
})

	t.Run("Get Sharers Document Not Found", func(t *testing.T) {
		// Attempt to get shares for a document ID that doesn't exist
		// ownerToken is defined in TestSharingEndpoints scope
		// Use a token available in this scope (e.g., token1 from TestDocumentHandlers setup)
		rr := performRequest(router, "GET", "/documents/non-existent-doc-id/shares", nil, token1)
		// This should trigger the !found check inside checkDocumentOwner
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "Document with ID 'non-existent-doc-id' not found")
	})
	t.Run("Delete Document Missing ID", func(t *testing.T) {
		// Request path is missing the document ID
		rr := performRequest(router, "DELETE", "/documents/", nil, token1)
		// Gin's routing will likely return 404 or 405 as no route matches DELETE /documents/
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rr.Code, "Requesting DELETE /documents/ should result in 404 or 405")
	})

}


// --- Sharing Endpoint Tests ---

func TestSharingEndpoints(t *testing.T) {
	router, database, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create users
	ownerID, _, ownerToken := createTestUserAndLogin(t, router, "owner@example.com", "ownerPass", "Doc", "Owner") // Get ownerID
	sharerID1, _, sharerToken1 := createTestUserAndLogin(t, router, "sharer1@example.com", "sharePass1", "Share", "User1") // Not used yet, just need ID
	sharerID2, _, _ := createTestUserAndLogin(t, router, "sharer2@example.com", "sharePass2", "Share", "User2")
	nonSharerID, _, nonSharerToken := createTestUserAndLogin(t, router, "nonsharer@example.com", "nonPassword1234", "Non", "Sharer") // Further increased password length

	// Owner creates a document
	docPayload := gin.H{"content": "Document to be shared"}
	rrCreate := performRequest(router, "POST", "/documents", marshalJSONBody(t, docPayload), ownerToken)
	require.Equal(t, http.StatusCreated, rrCreate.Code, "Document creation should return 201")
	var docResp map[string]interface{}
	err := json.Unmarshal(rrCreate.Body.Bytes(), &docResp)
	require.NoError(t, err)
	docID := docResp["id"].(string)
	require.NotEmpty(t, docID)

	shareBasePath := "/documents/" + docID + "/shares"

	// --- Initial State ---
	t.Run("Get Sharers Initial Empty", func(t *testing.T) {
		rr := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rr.Code)
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rr.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		assert.Empty(t, sharersResp.SharedWith, "Initially, shared_with should be empty")
	})

	t.Run("Get Sharers Not Owner", func(t *testing.T) {
		// Non-owner tries to get sharers
		rr := performRequest(router, "GET", shareBasePath, nil, nonSharerToken)
		// Expect 403 Forbidden or 404 Not Found (depends on handler checking ownership before checking doc existence)
		assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, rr.Code)
	})


	// --- PUT /documents/{id}/shares (Set/Replace) ---
	t.Run("Set Sharers Success", func(t *testing.T) {
		setPayload := gin.H{"shared_with": []string{sharerID1, sharerID2}}
		rr := performRequest(router, "PUT", shareBasePath, marshalJSONBody(t, setPayload), ownerToken)
		assert.Equal(t, http.StatusNoContent, rr.Code) // Expect 204 No Content

		// Verify with GET
		rrGet := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet.Code)
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rrGet.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{sharerID1, sharerID2}, sharersResp.SharedWith)

		// Verify database
		shareRecord, found := database.GetShareRecordByDocumentID(docID)
		require.True(t, found)
		assert.ElementsMatch(t, []string{sharerID1, sharerID2}, shareRecord.SharedWith)
	})

	t.Run("Set Sharers Not Owner", func(t *testing.T) {
		setPayload := gin.H{"shared_with": []string{nonSharerID}}
		rr := performRequest(router, "PUT", shareBasePath, marshalJSONBody(t, setPayload), nonSharerToken) // Use non-owner token
		assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, rr.Code)
	})

	t.Run("Set Sharers Empty List (Removes Shares)", func(t *testing.T) {
		setPayload := gin.H{"shared_with": []string{}}
		rr := performRequest(router, "PUT", shareBasePath, marshalJSONBody(t, setPayload), ownerToken)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		// Verify with GET
		rrGet := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet.Code)
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rrGet.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		// Check if the key exists but is empty, or if the key is omitted (depends on JSON marshaling)
		// Let's assume it returns an empty list if the record exists but is empty, or 404 if record deleted.
		// The DB implementation deletes the record if the list is empty. So handler should return 404? Let's check handler.
		// Assuming GetSharersHandler returns empty list if record not found for simplicity.
		assert.Empty(t, sharersResp.SharedWith)


		// Verify database (record should be deleted)
		_, found := database.GetShareRecordByDocumentID(docID)
		assert.False(t, found, "Share record should be deleted from DB after setting empty list")
	})

	t.Run("Set Sharers Missing Doc ID", func(t *testing.T) {
		setPayload := gin.H{"shared_with": []string{sharerID1}}
		// Request path is missing the document ID
		rr := performRequest(router, "PUT", "/documents//shares", marshalJSONBody(t, setPayload), ownerToken)
		// Gin's routing will likely return 404 as no route matches /documents//shares
		assert.Equal(t, http.StatusNotFound, rr.Code, "Requesting PUT /documents//shares should result in 404")
	})

	t.Run("Set Sharers Missing Body Field", func(t *testing.T) {
		// Send a request body without the required "shared_with" field
		setPayload := gin.H{"other_field": "value"}
		rr := performRequest(router, "PUT", shareBasePath, marshalJSONBody(t, setPayload), ownerToken)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Setting shares without 'shared_with' field should return 400 Bad Request")
	})

	t.Run("Set Sharers With Owner ID", func(t *testing.T) {
		// Attempt to share with the owner themselves
		setPayload := gin.H{"shared_with": []string{ownerID, sharerID1}}
		rr := performRequest(router, "PUT", shareBasePath, marshalJSONBody(t, setPayload), ownerToken)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Setting shares including owner ID should return 400 Bad Request")
		assert.Contains(t, rr.Body.String(), "Cannot share document with the owner")
	})

	t.Run("Set Sharers With Empty Profile ID", func(t *testing.T) {
		// Attempt to share with an empty string ID (should be skipped)
		setPayload := gin.H{"shared_with": []string{sharerID1, "", sharerID2}}
		rr := performRequest(router, "PUT", shareBasePath, marshalJSONBody(t, setPayload), ownerToken)
		assert.Equal(t, http.StatusNoContent, rr.Code, "Setting shares with an empty ID should succeed (empty ID skipped)")

		// Verify only valid IDs were added
		rrGet := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet.Code)
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rrGet.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{sharerID1, sharerID2}, sharersResp.SharedWith, "Share list should only contain non-empty IDs")
	})


	// --- PUT /documents/{id}/shares/{profile_id} (Add) ---
	t.Run("Add Sharer Success", func(t *testing.T) {
		// Reset shares first
		performRequest(router, "PUT", shareBasePath, marshalJSONBody(t, gin.H{"shared_with": []string{}}), ownerToken)

		// Add sharer1 back
		rr := performRequest(router, "PUT", shareBasePath+"/"+sharerID1, nil, ownerToken)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		// Verify with GET
		rrGet := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet.Code)
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rrGet.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		assert.Equal(t, []string{sharerID1}, sharersResp.SharedWith)

		// Add sharer2
		rr2 := performRequest(router, "PUT", shareBasePath+"/"+sharerID2, nil, ownerToken)
		assert.Equal(t, http.StatusNoContent, rr2.Code)

		// Verify with GET again
		rrGet2 := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet2.Code)
		var sharersResp2 struct{ SharedWith []string `json:"shared_with"`}
		err2 := json.Unmarshal(rrGet2.Body.Bytes(), &sharersResp2)
		require.NoError(t, err2)
		assert.ElementsMatch(t, []string{sharerID1, sharerID2}, sharersResp2.SharedWith)
	})

	t.Run("Add Sharer Not Owner", func(t *testing.T) {
		rr := performRequest(router, "PUT", shareBasePath+"/"+nonSharerID, nil, nonSharerToken) // Non-owner tries to add themselves
		assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, rr.Code)
	})

	t.Run("Add Sharer Already Exists", func(t *testing.T) {
		// sharerID1 is already added from previous test
		rr := performRequest(router, "PUT", shareBasePath+"/"+sharerID1, nil, ownerToken)
		assert.Equal(t, http.StatusNoContent, rr.Code) // Should still succeed idempotently

		// Verify list hasn't changed unexpectedly (no duplicates)
		rrGet := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet.Code)
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rrGet.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{sharerID1, sharerID2}, sharersResp.SharedWith)
	})

	t.Run("Add Sharer Missing IDs", func(t *testing.T) {
		// Missing profile_id
		rr1 := performRequest(router, "PUT", shareBasePath+"/", nil, ownerToken)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rr1.Code, "Requesting PUT .../shares/ should result in 404 or 405")

		// Missing doc id (will cause 404 on the base path)
		rr2 := performRequest(router, "PUT", "/documents//shares/"+sharerID1, nil, ownerToken)
		assert.Equal(t, http.StatusNotFound, rr2.Code, "Requesting PUT /documents//shares/... should result in 404")
	})

	t.Run("Add Sharer With Owner ID", func(t *testing.T) {
		// Attempt to add the owner themselves
		rr := performRequest(router, "PUT", shareBasePath+"/"+ownerID, nil, ownerToken)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "Adding owner ID as sharer should return 400 Bad Request")
		assert.Contains(t, rr.Body.String(), "Cannot share document with the owner")
	})


	// --- DELETE /documents/{id}/shares/{profile_id} (Remove) ---
	t.Run("Remove Sharer Success", func(t *testing.T) {
		// Remove sharer1
		rr := performRequest(router, "DELETE", shareBasePath+"/"+sharerID1, nil, ownerToken)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		// Verify with GET
		rrGet := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet.Code)
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rrGet.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		assert.Equal(t, []string{sharerID2}, sharersResp.SharedWith) // Only sharer2 should remain
	})

	t.Run("Remove Sharer Not Owner", func(t *testing.T) {
		// Non-owner tries to remove sharer2
		rr := performRequest(router, "DELETE", shareBasePath+"/"+sharerID2, nil, nonSharerToken)
		assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, rr.Code)

		// Verify sharer2 is still there
		rrGet := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet.Code)
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rrGet.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		assert.Equal(t, []string{sharerID2}, sharersResp.SharedWith)
	})

	t.Run("Remove Non-existent Sharer", func(t *testing.T) {
		// Try removing sharer1 again (already removed)
		rr := performRequest(router, "DELETE", shareBasePath+"/"+sharerID1, nil, ownerToken)
		assert.Equal(t, http.StatusNoContent, rr.Code) // Should succeed idempotently

		// Verify list hasn't changed
		rrGet := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet.Code)
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rrGet.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		assert.Equal(t, []string{sharerID2}, sharersResp.SharedWith)
	})

	t.Run("Remove Last Sharer (Deletes Record)", func(t *testing.T) {
		// Remove sharer2
		rr := performRequest(router, "DELETE", shareBasePath+"/"+sharerID2, nil, ownerToken)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		// Verify with GET (expect empty list or 404, see previous comment)
		rrGet := performRequest(router, "GET", shareBasePath, nil, ownerToken)
		assert.Equal(t, http.StatusOK, rrGet.Code) // Assuming handler returns OK with empty list
		var sharersResp struct{ SharedWith []string `json:"shared_with"`}
		err := json.Unmarshal(rrGet.Body.Bytes(), &sharersResp)
		require.NoError(t, err)
		assert.Empty(t, sharersResp.SharedWith)

		// Verify database (record should be deleted)
		_, found := database.GetShareRecordByDocumentID(docID)
		assert.False(t, found, "Share record should be deleted from DB after removing last sharer")
	})

	t.Run("Remove Sharer Missing IDs", func(t *testing.T) {
		// Missing profile_id
		rr1 := performRequest(router, "DELETE", shareBasePath+"/", nil, ownerToken)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rr1.Code, "Requesting DELETE .../shares/ should result in 404 or 405")

		// Missing doc id (will cause 404 on the base path)
		rr2 := performRequest(router, "DELETE", "/documents//shares/"+sharerID1, nil, ownerToken)
		assert.Equal(t, http.StatusNotFound, rr2.Code, "Requesting DELETE /documents//shares/... should result in 404")
	})

	// --- Access Control Check ---
	t.Run("Shared User Can Access Document", func(t *testing.T) {
		// Share doc with sharer1 again
		rrAdd := performRequest(router, "PUT", shareBasePath+"/"+sharerID1, nil, ownerToken)
		require.Equal(t, http.StatusNoContent, rrAdd.Code)

		// Sharer1 tries to GET the document
		rrGet := performRequest(router, "GET", "/documents/"+docID, nil, sharerToken1) // Use sharer1's token
		assert.Equal(t, http.StatusOK, rrGet.Code) // Should succeed

		var docResp map[string]interface{}
		err := json.Unmarshal(rrGet.Body.Bytes(), &docResp)
		require.NoError(t, err)
		assert.Equal(t, docID, docResp["id"])
	})

	t.Run("Non-Shared User Cannot Access Document", func(t *testing.T) {
		// NonSharer tries to GET the document
		rrGet := performRequest(router, "GET", "/documents/"+docID, nil, nonSharerToken)
		assert.Equal(t, http.StatusForbidden, rrGet.Code) // Expect 403 Forbidden
	})

}

// --- Password Reset Endpoint Tests ---

func TestPasswordResetEndpoints(t *testing.T) {
	router, database, _, cleanup := setupTestServer(t) // cfg is not used directly in this test
	defer cleanup()

	// Create a user for testing reset
	userID, userEmail, _ := createTestUserAndLogin(t, router, "reset.user@example.com", "initialPassword", "Reset", "User")
	require.NotEmpty(t, userID)

	var generatedOTP string // To store OTP between forgot and reset steps

	// --- Forgot Password ---
	t.Run("Forgot Password Success", func(t *testing.T) {
		forgotPayload := gin.H{"email": userEmail}
		rr := performRequest(router, "POST", "/auth/forgot-password", marshalJSONBody(t, forgotPayload), "")

		assert.Equal(t, http.StatusAccepted, rr.Code, "Forgot password should return 202 Accepted")

		// Verify OTP was stored in the database (need to retrieve it)
		otp, expiry, found := database.RetrieveOTP(userEmail) // Assuming RetrieveOTP is accessible
		assert.True(t, found, "OTP should be stored in the database")
		assert.NotEmpty(t, otp, "Stored OTP should not be empty")
		assert.True(t, expiry.After(time.Now()), "OTP expiry should be in the future")
		assert.Len(t, otp, 6, "OTP length should match expected") // Use literal 6 as otpLength is not exported

		generatedOTP = otp // Store for the reset test
	})

	t.Run("Forgot Password Non-Existent Email", func(t *testing.T) {
		forgotPayload := gin.H{"email": "nosuchuser@example.com"}
		rr := performRequest(router, "POST", "/auth/forgot-password", marshalJSONBody(t, forgotPayload), "")

		assert.Equal(t, http.StatusAccepted, rr.Code, "Forgot password for non-existent email should still return 202 Accepted")

		// Verify NO OTP was stored for this email
		_, _, found := database.RetrieveOTP("nosuchuser@example.com")
		assert.False(t, found, "OTP should NOT be stored for a non-existent email")
	})

	t.Run("Forgot Password Invalid Request", func(t *testing.T) {
		forgotPayload := gin.H{"email_address": userEmail} // Wrong field name
		rr := performRequest(router, "POST", "/auth/forgot-password", marshalJSONBody(t, forgotPayload), "")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})


	// --- Reset Password ---
	t.Run("Reset Password Invalid OTP", func(t *testing.T) {
		require.NotEmpty(t, generatedOTP, "Cannot run reset test without generated OTP")
		resetPayload := gin.H{
			"email":         userEmail,
			"otp":           "wrongOTP",
			"new_password":  "newPassword1",
		}
		rr := performRequest(router, "POST", "/auth/reset-password", marshalJSONBody(t, resetPayload), "")

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Reset password with invalid OTP should return 401 Unauthorized")

		// Verify password hasn't changed - try logging in with initial password
		loginPayload := gin.H{"email": userEmail, "password": "initialPassword"}
		loginRR := performRequest(router, "POST", "/auth/login", marshalJSONBody(t, loginPayload), "")
		assert.Equal(t, http.StatusOK, loginRR.Code, "Login with initial password should still work after failed reset attempt")
	})

	t.Run("Reset Password Expired OTP", func(t *testing.T) {
		// Generate a new OTP first
		forgotPayload := gin.H{"email": userEmail}
		performRequest(router, "POST", "/auth/forgot-password", marshalJSONBody(t, forgotPayload), "")
		otp, _, found := database.RetrieveOTP(userEmail)
		require.True(t, found)

		// Manually expire the OTP in the database
		database.StoreOTP(userEmail, otp, time.Now().Add(-1*time.Minute)) // Set expiry to 1 minute ago

		resetPayload := gin.H{
			"email":         userEmail,
			"otp":           otp, // Use the now-expired OTP
			"new_password":  "newPassword2",
		}
		rr := performRequest(router, "POST", "/auth/reset-password", marshalJSONBody(t, resetPayload), "")

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Reset password with expired OTP should return 401 Unauthorized")
		var errorResponse map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse["error"], "OTP has expired", "Error message should indicate OTP expiry")

		// Verify password hasn't changed
		loginPayload := gin.H{"email": userEmail, "password": "initialPassword"}
		loginRR := performRequest(router, "POST", "/auth/login", marshalJSONBody(t, loginPayload), "")
		assert.Equal(t, http.StatusOK, loginRR.Code, "Login with initial password should still work after expired OTP reset attempt")
	})


	t.Run("Reset Password Success", func(t *testing.T) {
		// Generate a fresh OTP
		forgotPayload := gin.H{"email": userEmail}
		performRequest(router, "POST", "/auth/forgot-password", marshalJSONBody(t, forgotPayload), "")
		otp, _, found := database.RetrieveOTP(userEmail)
		require.True(t, found)
		require.NotEmpty(t, otp)

		newPassword := "SuccessfullyResetPassword"
		resetPayload := gin.H{
			"email":         userEmail,
			"otp":           otp, // Use the fresh OTP
			"new_password":  newPassword,
		}
		rr := performRequest(router, "POST", "/auth/reset-password", marshalJSONBody(t, resetPayload), "")

		assert.Equal(t, http.StatusNoContent, rr.Code, "Successful password reset should return 204 No Content")

		// Verify OTP is deleted after successful reset
		_, _, foundAfterReset := database.RetrieveOTP(userEmail)
		assert.False(t, foundAfterReset, "OTP should be deleted after successful reset")

		// Verify password HAS changed - try logging in with the NEW password
		loginPayloadNew := gin.H{"email": userEmail, "password": newPassword}
		loginRRNew := performRequest(router, "POST", "/auth/login", marshalJSONBody(t, loginPayloadNew), "")
		assert.Equal(t, http.StatusOK, loginRRNew.Code, "Login with NEW password should work after successful reset")

		// Verify login with OLD password fails
		loginPayloadOld := gin.H{"email": userEmail, "password": "initialPassword"}
		loginRROld := performRequest(router, "POST", "/auth/login", marshalJSONBody(t, loginPayloadOld), "")
		assert.Equal(t, http.StatusUnauthorized, loginRROld.Code, "Login with OLD password should fail after successful reset")
	})

	t.Run("Reset Password OTP Not Found", func(t *testing.T) {
		// Assumes OTP was deleted by the successful reset test above
		resetPayload := gin.H{
			"email":         userEmail,
			"otp":           "anyOTP", // OTP doesn't exist anymore
			"new_password":  "newPassword3",
		}
		rr := performRequest(router, "POST", "/auth/reset-password", marshalJSONBody(t, resetPayload), "")

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Reset password with no OTP found should return 401 Unauthorized")
		var errorResponse map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse["error"], "no OTP found", "Error message should indicate OTP not found")
	})

	t.Run("Reset Password Invalid Request", func(t *testing.T) {
		resetPayload := gin.H{
			"email":         userEmail,
			// Missing OTP and new_password
		}
		rr := performRequest(router, "POST", "/auth/reset-password", marshalJSONBody(t, resetPayload), "")
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Reset Password User Not Found", func(t *testing.T) {
		// Create a temporary user for this specific test
		tempUserID, tempUserEmail, _ := createTestUserAndLogin(t, router, "temp.delete.reset@example.com", "tempPass", "Temp", "User")
		require.NotEmpty(t, tempUserID)

		// Generate OTP for the temporary user
		forgotPayload := gin.H{"email": tempUserEmail}
		performRequest(router, "POST", "/auth/forgot-password", marshalJSONBody(t, forgotPayload), "")
		otp, _, found := database.RetrieveOTP(tempUserEmail)
		require.True(t, found, "OTP should be generated for temp user")
		require.NotEmpty(t, otp)

		// Manually delete the user from the database AFTER generating OTP
		err := database.DeleteProfile(tempUserID)
		require.NoError(t, err, "Failed to manually delete temp user")

		// Attempt to reset password for the now-deleted user
		resetPayload := gin.H{
			"email":         tempUserEmail,
			"otp":           otp,
			"new_password":  "wontBeSetPassword",
		}
		rr := performRequest(router, "POST", "/auth/reset-password", marshalJSONBody(t, resetPayload), "")

		// Expect 404 Not Found because UpdateProfilePassword should fail for the deleted user
		assert.Equal(t, http.StatusNotFound, rr.Code, "Reset password for deleted user should return 404 Not Found")
	})

}