package utils

import (
	"docserver/config"
	"docserver/models" // Assuming models are needed for context, e.g., profile data
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	// "sync" // Removed unused import
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// --- Password Hashing ---

// HashPassword generates a bcrypt hash for the given password using the cost from config.
func HashPassword(password string, cost int) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		log.Printf("ERROR: Failed to hash password: %v", err)
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPasswordHash compares a plain text password with a stored bcrypt hash.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	// Returns nil on success, error on failure
	return err == nil
}

// --- JWT Handling ---

// Claims defines the structure of the JWT claims.
type Claims struct {
	UserID string `json:"user_id"` // Dashless UUID
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a new JWT token for a given user profile.
func GenerateJWT(profile *models.Profile, cfg *config.Config) (string, error) {
	if cfg.JwtSecret == "" {
		log.Println("CRITICAL: JWT Secret is empty. Cannot generate token.")
		return "", errors.New("JWT secret is not configured")
	}

	expirationTime := time.Now().Add(cfg.TokenLifetime)
	claims := &Claims{
		UserID: profile.ID, // Assumes profile.ID is already dashless
		Email:  profile.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "docserver", // As per plan
			Subject:   profile.ID,  // Often set to user ID
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.JwtSecret))
	if err != nil {
		log.Printf("ERROR: Failed to sign JWT token: %v", err)
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateJWT parses and validates a JWT token string.
// Returns the claims if valid, otherwise returns an error.
func ValidateJWT(tokenString string, cfg *config.Config) (*Claims, error) {
	if cfg.JwtSecret == "" {
		log.Println("CRITICAL: JWT Secret is empty. Cannot validate token.")
		return nil, errors.New("JWT secret is not configured")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is what we expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.JwtSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			log.Printf("INFO: JWT validation failed: Token expired")
			return nil, errors.New("token has expired")
		}
		log.Printf("WARN: JWT validation failed: %v", err)
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		log.Printf("WARN: JWT validation failed: Token marked as invalid")
		return nil, errors.New("invalid token")
	}

	// Check issuer?
	// if !claims.VerifyIssuer("docserver", true) {
	// 	return nil, errors.New("invalid token issuer")
	// }

	return claims, nil
}

// AuthMiddleware creates a Gin middleware function to protect routes.
// It validates the JWT token from the Authorization header.
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			GinUnauthorized(c, "Authorization header required")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			GinError(c, http.StatusBadRequest, "Authorization header format must be Bearer {token}")
			return
		}

		tokenString := parts[1]
		claims, err := ValidateJWT(tokenString, cfg)
		if err != nil {
			GinUnauthorized(c, fmt.Sprintf("Invalid token: %v", err))
			return
		}

		// Store user ID and email in context for handlers to use
		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email) // Add email as well, might be useful

		c.Next() // Proceed to the next handler
	}
}

// --- OTP Handling (for Password Reset) ---

// otpStore holds the temporary OTPs. In a real app, use Redis or similar.
// For this project, an in-memory map within the Database struct is used (defined in db/database.go).
// We need functions here to interact with that store via the Database instance.

const otpLifetime = 5 * time.Minute // OTP validity duration
const otpLength = 6                  // Length of the numeric OTP

// generateOTP creates a random numeric string of specified length.
func generateOTP(length int) string {
	const digits = "0123456789"
	otp := make([]byte, length)
	// Use crypto/rand for better randomness if needed, but math/rand is simpler for demo
	// Seed rand only once ideally (e.g., in main.go: rand.Seed(time.Now().UnixNano()))
	for i := range otp {
		otp[i] = digits[rand.Intn(len(digits))]
	}
	return string(otp)
}

// GenerateAndStoreOTP generates a new OTP, stores it temporarily (using the db's store),
// and returns the OTP. It should be printed to the console as per the plan.
// Note: This function needs access to the Database instance to store the OTP.
// It might be better placed as a method on the Database type in `db/database.go`.
// Let's define it here for now, assuming a db instance is passed.
func GenerateAndStoreOTP(email string, db interface { // Use interface to avoid circular dependency
	StoreOTP(email string, otp string, expiry time.Time)
}) (string, error) {
	otp := generateOTP(otpLength)
	expiry := time.Now().Add(otpLifetime)

	// Store the OTP using the passed database instance's method
	db.StoreOTP(email, otp, expiry)

	// Log clearly to console (as per plan)
	log.Printf("*****************************************************")
	log.Printf("PASSWORD RESET OTP GENERATED for %s: %s", email, otp)
	log.Printf("OTP expires at: %s", expiry.Format(time.RFC1123))
	log.Printf("*****************************************************")

	return otp, nil
}

// VerifyOTP checks if the provided OTP for the email is valid and not expired.
// Note: Needs access to the Database instance. Better as a method on Database.
func VerifyOTP(email, providedOTP string, db interface { // Use interface
	RetrieveOTP(email string) (string, time.Time, bool)
	DeleteOTP(email string)
}) (bool, error) {
	storedOTP, expiry, found := db.RetrieveOTP(email)

	if !found {
		return false, errors.New("no OTP found for this email or it has expired")
	}

	if time.Now().After(expiry) {
		// Clean up expired OTP
		db.DeleteOTP(email)
		return false, errors.New("OTP has expired")
	}

	if storedOTP != providedOTP {
		return false, errors.New("invalid OTP")
	}

	// OTP is valid, delete it after verification
	db.DeleteOTP(email)
	return true, nil
}

// --- Helper Methods for Database (to be implemented in db/database.go) ---
// These methods are needed by GenerateAndStoreOTP and VerifyOTP

// StoreOTP(email string, otp string, expiry time.Time)
// RetrieveOTP(email string) (otp string, expiry time.Time, found bool)
// DeleteOTP(email string)