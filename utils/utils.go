package utils

import (
	"github.com/google/uuid"
	"strings"
	"log"
	"net/http"
	"github.com/gin-gonic/gin"
)

// GenerateDashlessUUID creates a new UUID v4 and returns its string representation
// with all dashes removed.
func GenerateDashlessUUID() string {
	id := uuid.New()
	return strings.ReplaceAll(id.String(), "-", "")
}

// APIError is a standard structure for returning errors as JSON.
type APIError struct {
	Error string `json:"error"`
}

// GinError sends a JSON error response with a specific status code.
// It logs the error server-side as well.
func GinError(c *gin.Context, statusCode int, message string) {
	log.Printf("ERROR: Request %s %s - Status %d - %s", c.Request.Method, c.Request.URL.Path, statusCode, message)
	c.AbortWithStatusJSON(statusCode, APIError{Error: message})
}

// GinBadRequest sends a 400 Bad Request error response.
func GinBadRequest(c *gin.Context, message string) {
	GinError(c, http.StatusBadRequest, message)
}

// GinUnauthorized sends a 401 Unauthorized error response.
func GinUnauthorized(c *gin.Context, message string) {
	GinError(c, http.StatusUnauthorized, message)
}

// GinForbidden sends a 403 Forbidden error response.
func GinForbidden(c *gin.Context, message string) {
	GinError(c, http.StatusForbidden, message)
}

// GinNotFound sends a 404 Not Found error response.
func GinNotFound(c *gin.Context, message string) {
	GinError(c, http.StatusNotFound, message)
}

// GinInternalServerError sends a 500 Internal Server Error response.
func GinInternalServerError(c *gin.Context, message string) {
	GinError(c, http.StatusInternalServerError, message)
}

// Add other utility functions as needed...