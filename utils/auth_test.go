package utils

import (
	"docserver/config"
	"docserver/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	password := "mysecretpassword"
	cost := bcrypt.DefaultCost // Use default cost for testing

	hash, err := HashPassword(password, cost)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Error("Expected hash to not be empty")
	}

	// Try hashing again, should be different due to salt
	hash2, err := HashPassword(password, cost)
	if err != nil {
		t.Fatalf("HashPassword (2nd time) failed: %v", err)
	}
	if hash == hash2 {
		t.Error("Expected different hashes for the same password due to salt")
	}
}

func TestCheckPasswordHash(t *testing.T) {
	password := "mysecretpassword"
	wrongPassword := "wrongpassword"
	cost := bcrypt.DefaultCost

	hash, err := HashPassword(password, cost)
	if err != nil {
		t.Fatalf("Setup failed: HashPassword failed: %v", err)
	}

	// Test correct password
	if !CheckPasswordHash(password, hash) {
		t.Errorf("CheckPasswordHash should return true for the correct password")
	}

	// Test incorrect password
	if CheckPasswordHash(wrongPassword, hash) {
		t.Errorf("CheckPasswordHash should return false for an incorrect password")
	}

	// Test with potentially invalid hash (though bcrypt handles many cases)
	if CheckPasswordHash(password, "invalidhashstring") {
		t.Errorf("CheckPasswordHash should return false for an invalid hash format")
	}
}

// --- JWT Tests ---

// Helper function to create a basic config for testing JWT
func createTestJWTConfig() *config.Config {
	return &config.Config{
		JwtSecret:     "test-secret-key-longer-than-32-bytes", // Use a fixed secret for tests
		TokenLifetime: time.Hour,                              // Default lifetime
		// Other config fields can be zero/default values if not needed by JWT funcs
	}
}

// Helper function to create a basic profile for testing JWT
func createTestProfile() *models.Profile {
	return &models.Profile{
		ID:        GenerateDashlessUUID(), // Use the tested function
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
		// PasswordHash is not needed for JWT generation/validation tests
		CreationDate:   time.Now().UTC(),
		LastModifiedDate: time.Now().UTC(),
	}
}

func TestGenerateJWT(t *testing.T) {
	cfg := createTestJWTConfig()
	profile := createTestProfile()

	tokenString, err := GenerateJWT(profile, cfg)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if tokenString == "" {
		t.Error("Expected token string not to be empty")
	}

	// Basic check: contains three parts separated by dots
	if len(strings.Split(tokenString, ".")) != 3 {
		t.Errorf("Expected token string to have 3 parts, got: %s", tokenString)
	}

	// Test error case: Empty secret
	cfgEmptySecret := &config.Config{JwtSecret: "", TokenLifetime: time.Hour}
	_, err = GenerateJWT(profile, cfgEmptySecret)
	if err == nil {
		t.Error("Expected error when generating JWT with empty secret, but got nil")
	}
}

func TestValidateJWT(t *testing.T) {
	cfg := createTestJWTConfig()
	profile := createTestProfile()

	// 1. Test valid token
	validToken, err := GenerateJWT(profile, cfg)
	if err != nil {
		t.Fatalf("Setup failed: GenerateJWT failed: %v", err)
	}

	claims, err := ValidateJWT(validToken, cfg)
	if err != nil {
		t.Fatalf("ValidateJWT failed for valid token: %v", err)
	}
	if claims == nil {
		t.Fatal("ValidateJWT returned nil claims for valid token")
	}
	if claims.UserID != profile.ID {
		t.Errorf("Expected UserID %s, got %s", profile.ID, claims.UserID)
	}
	if claims.Email != profile.Email {
		t.Errorf("Expected Email %s, got %s", profile.Email, claims.Email)
	}
	if claims.Issuer != "docserver" {
		t.Errorf("Expected Issuer 'docserver', got %s", claims.Issuer)
	}

	// 2. Test invalid token string (malformed)
	_, err = ValidateJWT("this.is.not.a.valid.token", cfg)
	if err == nil {
		t.Error("Expected error when validating malformed token, but got nil")
	}

	// 3. Test token signed with different secret
	cfgWrongSecret := createTestJWTConfig()
	cfgWrongSecret.JwtSecret = "different-secret-key-also-needs-to-be-long"
	_, err = ValidateJWT(validToken, cfgWrongSecret)
	if err == nil {
		t.Error("Expected error when validating token with wrong secret, but got nil")
	} else if !strings.Contains(err.Error(), "invalid token") && !strings.Contains(err.Error(), "signature is invalid") {
		// Error message might vary slightly depending on jwt library version
		t.Errorf("Expected signature validation error, got: %v", err)
	}


	// 4. Test expired token
	cfgShortLived := createTestJWTConfig()
	cfgShortLived.TokenLifetime = -1 * time.Second // Expired 1 second ago
	expiredToken, err := GenerateJWT(profile, cfgShortLived)
	if err != nil {
		t.Fatalf("Setup failed: GenerateJWT for expired token failed: %v", err)
	}
	// time.Sleep(10 * time.Millisecond) // Ensure clock tick if needed, but negative duration should be enough
	_, err = ValidateJWT(expiredToken, cfg) // Validate against original config secret
	if err == nil {
		t.Error("Expected error when validating expired token, but got nil")
	} else if !strings.Contains(err.Error(), "token has expired") {
		t.Errorf("Expected 'token has expired' error, got: %v", err)
	}

	// 5. Test error case: Empty secret for validation
	cfgEmptySecret := &config.Config{JwtSecret: "", TokenLifetime: time.Hour}
	_, err = ValidateJWT(validToken, cfgEmptySecret)
	if err == nil {
		t.Error("Expected error when validating JWT with empty secret, but got nil")
	}
}

// --- OTP Tests ---

// Mock Database for OTP testing
type mockOtpDb struct {
	storedOtps map[string]struct {
		otp    string
		expiry time.Time
	}
	storeCalled   bool
	retrieveCalled bool
	deleteCalled  bool
	lastStoredEmail string
	lastStoredOtp   string
	lastStoredExpiry time.Time
	lastRetrievedEmail string
	lastDeletedEmail string
}

func newMockOtpDb() *mockOtpDb {
	return &mockOtpDb{
		storedOtps: make(map[string]struct {
			otp    string
			expiry time.Time
		}),
	}
}

// Mock implementation of StoreOTP
func (m *mockOtpDb) StoreOTP(email string, otp string, expiry time.Time) {
	m.storeCalled = true
	m.lastStoredEmail = email
	m.lastStoredOtp = otp
	m.lastStoredExpiry = expiry
	m.storedOtps[email] = struct {
		otp    string
		expiry time.Time
	}{otp, expiry}
}

// Mock implementation of RetrieveOTP
func (m *mockOtpDb) RetrieveOTP(email string) (string, time.Time, bool) {
	m.retrieveCalled = true
	m.lastRetrievedEmail = email
	data, found := m.storedOtps[email]
	if !found {
		return "", time.Time{}, false
	}
	return data.otp, data.expiry, true
}

// Mock implementation of DeleteOTP
func (m *mockOtpDb) DeleteOTP(email string) {
	m.deleteCalled = true
	m.lastDeletedEmail = email
	delete(m.storedOtps, email)
}

func TestGenerateOTP(t *testing.T) {
	otp := generateOTP(otpLength) // Use const from auth.go

	if len(otp) != otpLength {
		t.Errorf("Expected OTP length %d, got %d", otpLength, len(otp))
	}

	// Check if all characters are digits
	for _, char := range otp {
		if char < '0' || char > '9' {
			t.Errorf("Expected OTP to contain only digits, got %s", otp)
			break
		}
	}

	// Generate another one, should likely be different (though collisions are possible)
	otp2 := generateOTP(otpLength)
	if otp == otp2 {
		t.Logf("Warning: Generated two identical OTPs (%s), which is possible but unlikely.", otp)
	}
}


func TestGenerateAndStoreOTP(t *testing.T) {
	mockDb := newMockOtpDb()
	email := "otpuser@example.com"

	// Note: We don't capture log output here, but assume it works per auth.go
	generatedOtp, err := GenerateAndStoreOTP(email, mockDb)
	if err != nil {
		t.Fatalf("GenerateAndStoreOTP failed: %v", err)
	}

	if !mockDb.storeCalled {
		t.Error("Expected StoreOTP method on mock DB to be called")
	}
	if mockDb.lastStoredEmail != email {
		t.Errorf("Expected StoreOTP to be called with email %s, got %s", email, mockDb.lastStoredEmail)
	}
	if mockDb.lastStoredOtp != generatedOtp {
		t.Errorf("Expected StoreOTP to be called with generated OTP %s, got %s", generatedOtp, mockDb.lastStoredOtp)
	}
	if mockDb.lastStoredExpiry.IsZero() || !mockDb.lastStoredExpiry.After(time.Now()) {
		t.Errorf("Expected StoreOTP expiry time to be in the future, got %v", mockDb.lastStoredExpiry)
	}

	// Check if it's actually stored in the mock's internal map
	storedData, found := mockDb.storedOtps[email]
	if !found {
		t.Fatal("OTP was not found in the mock DB's internal map after StoreOTP call")
	}
	if storedData.otp != generatedOtp {
		t.Errorf("Stored OTP in mock map (%s) does not match generated OTP (%s)", storedData.otp, generatedOtp)
	}
}

func TestVerifyOTP(t *testing.T) {
	email := "verify@example.com"
	correctOtp := "123456"
	wrongOtp := "987654"
	validExpiry := time.Now().Add(5 * time.Minute)
	expiredExpiry := time.Now().Add(-5 * time.Minute) // Expired 5 mins ago

	// --- Test Cases ---

	// 1. Valid OTP, not expired
	t.Run("ValidOTP_NotExpired", func(t *testing.T) {
		mockDb := newMockOtpDb() // Reset mock for isolation
		mockDb.StoreOTP(email, correctOtp, validExpiry)
		mockDb.deleteCalled = false // Reset delete flag

		valid, err := VerifyOTP(email, correctOtp, mockDb)
		if err != nil {
			t.Errorf("Expected no error for valid OTP, got: %v", err)
		}
		if !valid {
			t.Error("Expected verification to return true for valid OTP")
		}
		if !mockDb.deleteCalled {
			t.Error("Expected DeleteOTP to be called after successful verification")
		}
		if mockDb.lastDeletedEmail != email {
			t.Errorf("Expected DeleteOTP to be called with email %s, got %s", email, mockDb.lastDeletedEmail)
		}
		// Check if actually deleted
		if _, _, found := mockDb.RetrieveOTP(email); found {
			t.Error("OTP should have been deleted from mock store after successful verification")
		}
	})

	// 2. Invalid OTP
	t.Run("InvalidOTP", func(t *testing.T) {
		mockDb := newMockOtpDb()
		mockDb.StoreOTP(email, correctOtp, validExpiry)
		mockDb.deleteCalled = false

		valid, err := VerifyOTP(email, wrongOtp, mockDb)
		if err == nil {
			t.Error("Expected error for invalid OTP, got nil")
		} else if !strings.Contains(err.Error(), "invalid OTP") {
			t.Errorf("Expected error message 'invalid OTP', got: %v", err)
		}
		if valid {
			t.Error("Expected verification to return false for invalid OTP")
		}
		if mockDb.deleteCalled {
			t.Error("Expected DeleteOTP NOT to be called after failed verification (invalid OTP)")
		}
	})

	// 3. Correct OTP, but expired
	t.Run("CorrectOTP_Expired", func(t *testing.T) {
		mockDb := newMockOtpDb()
		mockDb.StoreOTP(email, correctOtp, expiredExpiry)
		mockDb.deleteCalled = false

		valid, err := VerifyOTP(email, correctOtp, mockDb)
		if err == nil {
			t.Error("Expected error for expired OTP, got nil")
		} else if !strings.Contains(err.Error(), "OTP has expired") {
			t.Errorf("Expected error message 'OTP has expired', got: %v", err)
		}
		if valid {
			t.Error("Expected verification to return false for expired OTP")
		}
		if !mockDb.deleteCalled {
			t.Error("Expected DeleteOTP to be called after expiry check")
		}
		if mockDb.lastDeletedEmail != email {
			t.Errorf("Expected DeleteOTP to be called with email %s, got %s", email, mockDb.lastDeletedEmail)
		}
		// Check if actually deleted
		if _, _, found := mockDb.RetrieveOTP(email); found {
			t.Error("Expired OTP should have been deleted from mock store")
		}
	})

	// 4. No OTP found for email
	t.Run("NoOTPFound", func(t *testing.T) {
		mockDb := newMockOtpDb() // Create mock inside subtest (empty store)

		valid, err := VerifyOTP(email, correctOtp, mockDb)
		if err == nil {
			t.Error("Expected error when no OTP found, got nil")
		} else if !strings.Contains(err.Error(), "no OTP found") {
			t.Errorf("Expected error message 'no OTP found', got: %v", err)
		}
		if valid {
			t.Error("Expected verification to return false when no OTP found")
		}
		if mockDb.deleteCalled {
			t.Error("Expected DeleteOTP NOT to be called when no OTP was found")
		}
	})
}

// --- AuthMiddleware Tests ---

func TestAuthMiddleware(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	cfg := createTestJWTConfig()
	profile := createTestProfile()
	validToken, _ := GenerateJWT(profile, cfg)

	cfgExpired := createTestJWTConfig()
	cfgExpired.TokenLifetime = -time.Hour // Expired token
	expiredToken, _ := GenerateJWT(profile, cfgExpired)

	cfgWrongSecret := createTestJWTConfig()
	cfgWrongSecret.JwtSecret = "a-completely-different-wrong-secret-key"
	tokenWrongSecret, _ := GenerateJWT(profile, cfgWrongSecret) // Generate with correct config first
	// We will validate tokenWrongSecret against the original 'cfg' to simulate wrong secret


	// Test Handler to check if middleware allows request through
	testHandler := func(c *gin.Context) {
		userID, exists := c.Get("userID")
		assert.True(t, exists, "userID should exist in context")
		assert.Equal(t, profile.ID, userID, "userID in context should match profile ID")

		userEmail, exists := c.Get("userEmail")
		assert.True(t, exists, "userEmail should exist in context")
		assert.Equal(t, profile.Email, userEmail, "userEmail in context should match profile email")

		c.Status(http.StatusOK) // Indicate success
	}

	// Create router with middleware
	router := gin.New() // Use New instead of Default to avoid default middleware
	router.Use(AuthMiddleware(cfg))
	router.GET("/protected", testHandler)

	// --- Test Cases ---

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectBody     bool // Whether to check for APIError in body
		expectedError  string // Substring of expected error message if expectBody is true
		expectNext     bool   // Whether the testHandler should be called
	}{
		{
			name:           "No Auth Header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectBody:     true,
			expectedError:  "Authorization header required",
			expectNext:     false,
		},
		{
			name:           "Malformed Header - No Bearer",
			authHeader:     validToken, // Just the token, no "Bearer" prefix
			expectedStatus: http.StatusBadRequest,
			expectBody:     true,
			expectedError:  "Authorization header format must be Bearer {token}",
			expectNext:     false,
		},
		{
			name:           "Malformed Header - Wrong Scheme",
			authHeader:     "Basic " + validToken,
			expectedStatus: http.StatusBadRequest,
			expectBody:     true,
			expectedError:  "Authorization header format must be Bearer {token}",
			expectNext:     false,
		},
		{
			name:           "Malformed Header - Too Many Parts",
			authHeader:     "Bearer " + validToken + " extra",
			expectedStatus: http.StatusBadRequest,
			expectBody:     true,
			expectedError:  "Authorization header format must be Bearer {token}",
			expectNext:     false,
		},
		{
			name:           "Invalid Token - Wrong Secret",
			authHeader:     "Bearer " + tokenWrongSecret, // Use token generated with correct secret, validated against wrong one
			expectedStatus: http.StatusUnauthorized,
			expectBody:     true,
			expectedError:  "Invalid token: invalid token: token signature is invalid: signature is invalid", // Error from ValidateJWT, wrapped by middleware
			expectNext:     false,
		},
		{
			name:           "Invalid Token - Expired",
			authHeader:     "Bearer " + expiredToken,
			expectedStatus: http.StatusUnauthorized,
			expectBody:     true,
			expectedError:  "token has expired", // Error from ValidateJWT
			expectNext:     false,
		},
		{
			name:           "Valid Token",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK, // Expect the handler to run and return OK
			expectBody:     false,
			expectNext:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Status code mismatch")

			if tt.expectBody {
				var response APIError // Use APIError struct from utils
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err, "Failed to unmarshal error response")
				assert.Contains(t, response.Error, tt.expectedError, "Error message mismatch")
			}

			// Check if handler was called (only for the valid case)
			if tt.expectNext {
				// Assertions inside testHandler cover context values
				assert.Equal(t, http.StatusOK, w.Code, "Handler should return OK for valid token")
			} else {
				// Ensure handler didn't accidentally return OK status on error
				assert.NotEqual(t, http.StatusOK, w.Code, "Handler should not return OK on auth failure")
			}
		})
	}
}