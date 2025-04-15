package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGenerateDashlessUUID(t *testing.T) {
	uuid := GenerateDashlessUUID()

	// Check length (should be 32 characters)
	if len(uuid) != 32 {
		t.Errorf("Expected UUID length 32, got %d", len(uuid))
	}

	// Check for dashes
	if strings.Contains(uuid, "-") {
		t.Errorf("Generated UUID should not contain dashes, got %s", uuid)
	}

	// Optional: Could add a regex check for hex characters, but length and no dashes is usually sufficient
	// match, _ := regexp.MatchString(`^[a-f0-9]{32}$`, uuid)
	// if !match {
	// 	t.Errorf("Generated UUID does not match expected format: %s", uuid)
	// }
}

// Helper function to create a test Gin context
func createTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/test", nil) // Add a dummy request
	return c, w
}

func TestGinError(t *testing.T) {
	c, w := createTestContext()
	testMsg := "Generic error"
	testCode := http.StatusTeapot // Use a distinct code

	GinError(c, testCode, testMsg)

	assert.Equal(t, testCode, w.Code)

	var response APIError
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, testMsg, response.Error)
	assert.True(t, c.IsAborted(), "Context should be aborted")
}

func TestGinErrorHelpers(t *testing.T) {
	testCases := []struct {
		name       string
		helperFunc func(*gin.Context, string)
		wantCode   int
		wantMsg    string
	}{
		{
			name:       "BadRequest",
			helperFunc: GinBadRequest,
			wantCode:   http.StatusBadRequest,
			wantMsg:    "Bad request test",
		},
		{
			name:       "Unauthorized",
			helperFunc: GinUnauthorized,
			wantCode:   http.StatusUnauthorized,
			wantMsg:    "Unauthorized test",
		},
		{
			name:       "Forbidden",
			helperFunc: GinForbidden,
			wantCode:   http.StatusForbidden,
			wantMsg:    "Forbidden test",
		},
		{
			name:       "NotFound",
			helperFunc: GinNotFound,
			wantCode:   http.StatusNotFound,
			wantMsg:    "Not found test",
		},
		{
			name:       "InternalServerError",
			helperFunc: GinInternalServerError,
			wantCode:   http.StatusInternalServerError,
			wantMsg:    "Internal server error test",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := createTestContext()
			tc.helperFunc(c, tc.wantMsg)

			assert.Equal(t, tc.wantCode, w.Code)

			var response APIError
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantMsg, response.Error)
			assert.True(t, c.IsAborted(), "Context should be aborted")
		})
	}
}