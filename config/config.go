package config

import (
	"flag"
	"os"
	"path/filepath"
	"time"
	"log"
	// "crypto/rand" // No longer needed after removing JWT generation
	// "encoding/hex"  // No longer needed after removing JWT generation
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
	// Priority: File (CLI/Env) > Env Var > Generate
	if cfg.JwtSecretFile != "" {
		// Load from file specified by flag/env
		secretBytes, err := os.ReadFile(cfg.JwtSecretFile)
		if err != nil {
			log.Printf("ERROR: Failed to read JWT secret from file '%s': %v. Attempting fallback/generation.", cfg.JwtSecretFile, err)
			// Fall through to check ENV or generate
		} else {
			cfg.JwtSecret = string(secretBytes)
			log.Printf("INFO: Loaded JWT secret from file: %s", cfg.JwtSecretFile)
		}
	}

	// If file wasn't specified or failed to load, check environment variable
	if cfg.JwtSecret == "" {
		// Use DOCSERVER_ prefix for environment variable
		cfg.JwtSecret = getEnv("DOCSERVER_JWT_SECRET", defaultJwtSecretEnv)
		if cfg.JwtSecret != "" {
			log.Printf("INFO: Loaded JWT secret from DOCSERVER_JWT_SECRET environment variable.")
		}
	}

	// If still no secret after checking file and env var, it's a fatal error.
	if cfg.JwtSecret == "" {
		// Removed automatic generation. A secret MUST be provided.
		return nil, fmt.Errorf("JWT secret not provided via file (DOCSERVER_JWT_SECRET_FILE) or environment variable (DOCSERVER_JWT_SECRET)")
	}
	// Basic validation: ensure secret is not just whitespace
	if strings.TrimSpace(cfg.JwtSecret) == "" {
		return nil, fmt.Errorf("JWT secret cannot be empty or only whitespace")
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


	logConfiguration(cfg) // Log the final configuration

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
func logConfiguration(cfg *Config) {
	log.Println("--- Configuration ---")
	log.Printf("Server Address: %s", cfg.ListenAddress)
	log.Printf("Server Port: %s", cfg.ListenPort)
	log.Printf("Database File: %s", cfg.DbFilePath)
	log.Printf("Database Save Interval: %s", cfg.SaveInterval)
	log.Printf("Database Backup Enabled: %t", cfg.EnableBackup)
	log.Printf("JWT Secret Source: %s", determineJwtSecretSource(cfg))
	log.Printf("JWT Token Lifetime: %s", cfg.TokenLifetime)
	log.Printf("Bcrypt Cost: %d", cfg.BcryptCost)
	log.Println("---------------------")
}

// determineJwtSecretSource provides a string indicating how the JWT secret was obtained.
func determineJwtSecretSource(cfg *Config) string {
	if cfg.JwtSecretFile != "" {
		// Check if the secret actually came from this file (it might have failed loading)
		secretBytes, err := os.ReadFile(cfg.JwtSecretFile)
		if err == nil && string(secretBytes) == cfg.JwtSecret {
			return fmt.Sprintf("File (%s)", cfg.JwtSecretFile)
		}
	}
	// Use DOCSERVER_ prefix
	if envSecret := getEnv("DOCSERVER_JWT_SECRET", ""); envSecret != "" && envSecret == cfg.JwtSecret {
		return "Environment Variable (DOCSERVER_JWT_SECRET)"
	}
	// Removed check for generated key file as generation is removed
	// If it wasn't from the specified file or the env var, the source is unclear or potentially problematic
	// but LoadConfig should have already returned an error if it was empty.
	// This function might need adjustment if more complex secret sources are added later.
	return "Provided (File or Environment)"
}

// Helper function to handle errors during config loading - could be expanded
func handleConfigError(field string, value string, err error, defaultValue any) {
    log.Printf("WARN: Invalid value for %s: '%s'. Using default %v. Error: %v", field, value, defaultValue, err)
}
