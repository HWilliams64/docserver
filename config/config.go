package config

import (
	"flag"
	"os"
	"path/filepath"
	"time"
	"log"
	"crypto/rand" // Needed for JWT generation
	"encoding/hex"  // Needed for JWT generation
	"fmt"
	"strings"
)

// Config holds all configuration settings for the application.
type Config struct {
	// Server settings
	ListenAddress string
	ListenPort    string

	// Database settings
	DbFilePath    string
	SaveInterval  time.Duration
	EnableBackup  bool

	// Authentication settings
	JwtSecret     string // The actual secret key
	JwtSecretFile string // Path to the file containing the secret
	TokenLifetime time.Duration
	BcryptCost    int
}

const (
	defaultAddress       = "0.0.0.0"
	defaultPort          = "8080"
	defaultDbFile        = "./docs.json" // Relative to working dir
	defaultSaveInterval  = 3 * time.Second
	defaultEnableBackup  = true
	defaultJwtSecretFile = "" // No default file
	defaultJwtSecretEnv  = "" // No default env secret
	defaultJwtKeyFile    = "./docs.key" // Default file if we generate a key
	defaultTokenLifetime = 1 * time.Hour
	defaultBcryptCost    = 12
)

// LoadConfig loads configuration from defaults, environment variables, and command-line flags.
// Command-line flags take precedence over environment variables, which take precedence over defaults.
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// Define flags
	// Use DOCSERVER_ prefix for environment variables to align with testing and avoid conflicts
	flag.StringVar(&cfg.ListenAddress, "address", getEnv("DOCSERVER_LISTEN_ADDRESS", defaultAddress), "Server listen address (Env: DOCSERVER_LISTEN_ADDRESS)")
	// Define flag with the ultimate default. We'll check env var after parsing.
	flag.StringVar(&cfg.ListenPort, "port", defaultPort, "Server listen port (Env: DOCSERVER_LISTEN_PORT)")
	flag.StringVar(&cfg.DbFilePath, "db-file", getEnv("DOCSERVER_DB_FILE_PATH", defaultDbFile), "Path to the JSON database file (Env: DOCSERVER_DB_FILE_PATH)")
	saveIntervalStr := flag.String("save-interval", getEnv("DOCSERVER_SAVE_INTERVAL", defaultSaveInterval.String()), "Debounce interval for saving DB (e.g., 5s, 100ms) (Env: DOCSERVER_SAVE_INTERVAL)")
	flag.BoolVar(&cfg.EnableBackup, "enable-backup", getEnvBool("DOCSERVER_ENABLE_BACKUP", defaultEnableBackup), "Enable database backup (.bak file) before saving (Env: DOCSERVER_ENABLE_BACKUP)")
	flag.StringVar(&cfg.JwtSecretFile, "jwt-secret-file", getEnv("DOCSERVER_JWT_SECRET_FILE", defaultJwtSecretFile), "Path to file containing JWT secret key (overrides DOCSERVER_JWT_SECRET env var) (Env: DOCSERVER_JWT_SECRET_FILE)")

	// Non-configurable defaults (as per plan)
	cfg.TokenLifetime = defaultTokenLifetime
	cfg.BcryptCost = defaultBcryptCost

	// Parse flags to override defaults and env vars
	flag.Parse()

	// --- Post-Flag Parsing Adjustments ---
	// Explicitly check environment variables to allow them to override defaults
	// if the corresponding flag was not provided.

	// Port
	envPort := getEnv("DOCSERVER_LISTEN_PORT", "")
	// If the flag wasn't set (still default) AND the env var exists, use the env var.
	if cfg.ListenPort == defaultPort && envPort != "" {
		cfg.ListenPort = envPort
	}

	// DbFilePath (similar logic)
	envDbFile := getEnv("DOCSERVER_DB_FILE_PATH", "")
	if cfg.DbFilePath == defaultDbFile && envDbFile != "" {
		cfg.DbFilePath = envDbFile
	}

	// SaveInterval (needs parsing)
	envSaveInterval := getEnv("DOCSERVER_SAVE_INTERVAL", "")
	// If the flag wasn't set (still default) AND the env var exists, try parsing env var.
	if *saveIntervalStr == defaultSaveInterval.String() && envSaveInterval != "" {
		// Try parsing the environment variable duration
		_, err := time.ParseDuration(envSaveInterval) // Use blank identifier
		if err == nil {
			*saveIntervalStr = envSaveInterval // Update the string for later parsing logic if valid
		} else {
			log.Printf("WARN: Invalid duration in DOCSERVER_SAVE_INTERVAL: '%s'. Using default/flag value. Error: %v", envSaveInterval, err)
		}
	}
// EnableBackup (boolean) - No post-parsing check needed.
// The initial flag definition `flag.BoolVar(&cfg.EnableBackup, "enable-backup", getEnvBool("DOCSERVER_ENABLE_BACKUP", defaultEnableBackup), ...)`
// correctly handles the environment variable override when the flag isn't explicitly set.

	// JwtSecretFile (similar logic to DbFilePath)
	envJwtSecretFile := getEnv("DOCSERVER_JWT_SECRET_FILE", "")
	if cfg.JwtSecretFile == defaultJwtSecretFile && envJwtSecretFile != "" {
		cfg.JwtSecretFile = envJwtSecretFile
	}


	// Parse duration after flags are parsed
	var err error
	cfg.SaveInterval, err = time.ParseDuration(*saveIntervalStr)
	if err != nil {
		log.Printf("WARN: Invalid save-interval duration '%s'. Using default %s. Error: %v", *saveIntervalStr, defaultSaveInterval, err)
		cfg.SaveInterval = defaultSaveInterval
	}

	// --- JWT Secret Handling ---
	// Priority: File (CLI/Env) > Env Var > Default Key File > Generate
	var secretSource string // To track where the secret came from for logging

	// 1. Check explicit file path (from flag or DOCSERVER_JWT_SECRET_FILE env)
	if cfg.JwtSecretFile != "" {
		secretBytes, err := os.ReadFile(cfg.JwtSecretFile)
		if err == nil {
			cfg.JwtSecret = strings.TrimSpace(string(secretBytes))
			if cfg.JwtSecret != "" {
				log.Printf("INFO: Loaded JWT secret from specified file: %s", cfg.JwtSecretFile)
				secretSource = fmt.Sprintf("File (%s)", cfg.JwtSecretFile)
			} else {
				log.Printf("WARN: Specified JWT secret file '%s' is empty or contains only whitespace. Ignoring.", cfg.JwtSecretFile)
			}
		} else {
			log.Printf("WARN: Failed to read specified JWT secret file '%s': %v. Checking other sources.", cfg.JwtSecretFile, err)
		}
	}

	// 2. Check environment variable (DOCSERVER_JWT_SECRET) if not loaded from file
	if cfg.JwtSecret == "" {
		envSecret := getEnv("DOCSERVER_JWT_SECRET", defaultJwtSecretEnv)
		cfg.JwtSecret = strings.TrimSpace(envSecret)
		if cfg.JwtSecret != "" {
			log.Printf("INFO: Loaded JWT secret from DOCSERVER_JWT_SECRET environment variable.")
			secretSource = "Environment Variable (DOCSERVER_JWT_SECRET)"
		}
	}

	// 3. Check default key file (./docs.key) if still no secret
	if cfg.JwtSecret == "" {
		secretBytes, err := os.ReadFile(defaultJwtKeyFile)
		if err == nil {
			cfg.JwtSecret = strings.TrimSpace(string(secretBytes))
			if cfg.JwtSecret != "" {
				log.Printf("INFO: Loaded JWT secret from default key file: %s", defaultJwtKeyFile)
				secretSource = fmt.Sprintf("Default Key File (%s)", defaultJwtKeyFile)
			} else {
				log.Printf("WARN: Default JWT key file '%s' is empty or contains only whitespace. Will attempt generation.", defaultJwtKeyFile)
			}
		} else if !os.IsNotExist(err) {
			// File exists but couldn't be read
			log.Printf("WARN: Failed to read default JWT key file '%s': %v. Will attempt generation.", defaultJwtKeyFile, err)
		}
		// If err is os.IsNotExist, we proceed to generation silently.
	}

	// 4. Generate secret if still not found and save to default file
	if cfg.JwtSecret == "" {
		log.Printf("INFO: JWT secret not found via file, environment variable, or default key file. Generating a new secret...")
		newSecret, err := generateRandomKey(32) // Generate a 256-bit key
		if err != nil {
			return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		cfg.JwtSecret = newSecret

		// Attempt to save the generated key to the default file
		err = os.WriteFile(defaultJwtKeyFile, []byte(newSecret), 0600) // Read/write for owner only
		if err != nil {
			// Log a warning but continue - the server can still run with the generated key in memory
			log.Printf("WARN: Failed to save generated JWT secret to '%s': %v. The server will use the generated key for this session only.", defaultJwtKeyFile, err)
		} else {
			log.Printf("INFO: Successfully generated and saved new JWT secret to: %s", defaultJwtKeyFile)
			secretSource = fmt.Sprintf("Generated & Saved (%s)", defaultJwtKeyFile)
		}
	}

	// Final validation: ensure secret is not empty after all checks/generation
	if cfg.JwtSecret == "" {
		// This should theoretically not happen if generation works, but good to have a safeguard.
		return nil, fmt.Errorf("failed to obtain a valid JWT secret after checking all sources and attempting generation")
	}

	// --- Database Path Validation ---
	// Ensure DbFilePath is absolute or relative to the current working directory
	absDbPath, err := filepath.Abs(cfg.DbFilePath)
	if err != nil {
		// If we can't even get an absolute path, treat it as an error
		return nil, fmt.Errorf("could not determine absolute path for db-file '%s': %w", cfg.DbFilePath, err)
		// log.Printf("WARN: Could not determine absolute path for db-file '%s': %v. Using provided path.", cfg.DbFilePath, err)
	} else {
		cfg.DbFilePath = absDbPath
	}

	// Check if the resolved DB path points to an existing directory
	fileInfo, err := os.Stat(cfg.DbFilePath)
	if err == nil && fileInfo.IsDir() { // Path exists and it's a directory
		return nil, fmt.Errorf("database path '%s' points to a directory, not a file", cfg.DbFilePath)
	}
	// We don't return os.IsNotExist(err) as an error here, because the DB might be created on first run.
	// Further validation (permissions, etc.) will happen in db.NewDatabase.

	// (Moved path resolution and validation earlier, before logging)


	logConfiguration(cfg, secretSource) // Log the final configuration, passing the source hint

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvBool retrieves a boolean environment variable or returns a default value.
// Recognizes "true", "1", "yes" (case-insensitive) as true.
func getEnvBool(key string, fallback bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		switch strings.ToLower(value) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
		log.Printf("WARN: Invalid boolean value for environment variable %s: '%s'. Using default: %t", key, value, fallback)
	}
	return fallback
}

// logConfiguration prints the loaded configuration settings.
// Takes secretSource hint from LoadConfig.
func logConfiguration(cfg *Config, secretSource string) {
	log.Println("--- Configuration ---")
	log.Printf("Server Address: %s", cfg.ListenAddress)
	log.Printf("Server Port: %s", cfg.ListenPort)
	log.Printf("Database File: %s", cfg.DbFilePath)
	log.Printf("Database Save Interval: %s", cfg.SaveInterval)
	log.Printf("Database Backup Enabled: %t", cfg.EnableBackup)
	log.Printf("JWT Secret Source: %s", determineJwtSecretSource(cfg, secretSource)) // Pass hint
	log.Printf("JWT Token Lifetime: %s", cfg.TokenLifetime)
	log.Printf("Bcrypt Cost: %d", cfg.BcryptCost)
	log.Println("---------------------")
}

// determineJwtSecretSource provides a string indicating how the JWT secret was obtained.
// It relies on the secretSource variable being set correctly during LoadConfig.
// Note: This function is called *after* LoadConfig successfully completes.
func determineJwtSecretSource(cfg *Config, sourceHint string) string {
	// If LoadConfig provided a hint, use it.
	if sourceHint != "" {
		return sourceHint
	}

	// Fallback logic if sourceHint wasn't passed (shouldn't happen ideally)
	// This logic might be slightly less accurate if errors occurred during loading.
	log.Println("WARN: determineJwtSecretSource called without source hint, attempting fallback detection.")
	if cfg.JwtSecretFile != "" {
		secretBytes, err := os.ReadFile(cfg.JwtSecretFile)
		if err == nil && strings.TrimSpace(string(secretBytes)) == cfg.JwtSecret {
			return fmt.Sprintf("File (%s)", cfg.JwtSecretFile)
		}
	}
	if envSecret := getEnv("DOCSERVER_JWT_SECRET", ""); envSecret != "" && strings.TrimSpace(envSecret) == cfg.JwtSecret {
		return "Environment Variable (DOCSERVER_JWT_SECRET)"
	}
	secretBytes, err := os.ReadFile(defaultJwtKeyFile)
	if err == nil && strings.TrimSpace(string(secretBytes)) == cfg.JwtSecret {
		// Check if it was potentially generated in this run vs. pre-existing
		// This distinction is hard without more state, assume pre-existing if found here.
		return fmt.Sprintf("Default Key File (%s)", defaultJwtKeyFile)
	}

	// If none of the above match, it was likely generated in memory (maybe save failed)
	return "Generated (In Memory)"
}

// generateRandomKey generates a cryptographically secure random key of the specified byte length
// and returns it as a hex-encoded string.
func generateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// Helper function to handle errors during config loading - could be expanded
func handleConfigError(field string, value string, err error, defaultValue any) {
    log.Printf("WARN: Invalid value for %s: '%s'. Using default %v. Error: %v", field, value, defaultValue, err)
}
