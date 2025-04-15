package config

import (
	"flag"
	"os"
	"path/filepath"
	"time"
	"log"
	"crypto/rand"
	"encoding/hex"
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
	flag.StringVar(&cfg.ListenAddress, "address", getEnv("ADDRESS", defaultAddress), "Server listen address (Env: ADDRESS)")
	flag.StringVar(&cfg.ListenPort, "port", getEnv("PORT", defaultPort), "Server listen port (Env: PORT)")
	flag.StringVar(&cfg.DbFilePath, "db-file", getEnv("DB_FILE", defaultDbFile), "Path to the JSON database file (Env: DB_FILE)")
	saveIntervalStr := flag.String("save-interval", getEnv("SAVE_INTERVAL", defaultSaveInterval.String()), "Debounce interval for saving DB (e.g., 5s, 100ms) (Env: SAVE_INTERVAL)")
	flag.BoolVar(&cfg.EnableBackup, "enable-backup", getEnvBool("ENABLE_BACKUP", defaultEnableBackup), "Enable database backup (.bak file) before saving (Env: ENABLE_BACKUP)")
	flag.StringVar(&cfg.JwtSecretFile, "jwt-secret-file", getEnv("JWT_SECRET_FILE", defaultJwtSecretFile), "Path to file containing JWT secret key (overrides JWT_SECRET env var) (Env: JWT_SECRET_FILE)")

	// Non-configurable defaults (as per plan)
	cfg.TokenLifetime = defaultTokenLifetime
	cfg.BcryptCost = defaultBcryptCost

	// Parse flags to override defaults and env vars
	flag.Parse()

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
		cfg.JwtSecret = getEnv("JWT_SECRET", defaultJwtSecretEnv)
		if cfg.JwtSecret != "" {
			log.Printf("INFO: Loaded JWT secret from JWT_SECRET environment variable.")
		}
	}

	// If still no secret, generate one and save it
	if cfg.JwtSecret == "" {
		log.Printf("INFO: No JWT secret provided via file or environment variable. Generating a new secret.")
		newSecretBytes := make([]byte, 32) // 256 bits
		if _, err := rand.Read(newSecretBytes); err != nil {
			return nil, fmt.Errorf("failed to generate random JWT secret: %w", err)
		}
		cfg.JwtSecret = hex.EncodeToString(newSecretBytes)

		// Attempt to save the generated key to the default file
		keyFilePath := defaultJwtKeyFile
		log.Printf("INFO: Saving generated JWT secret to: %s", keyFilePath)
		// Ensure directory exists (though it's likely '.')
		if err := os.MkdirAll(filepath.Dir(keyFilePath), 0750); err != nil {
			log.Printf("WARN: Could not create directory for JWT key file '%s': %v. Secret will only be in memory.", keyFilePath, err)
		} else {
			// Write the file (permissions 0600: owner read/write only)
			if err := os.WriteFile(keyFilePath, []byte(cfg.JwtSecret), 0600); err != nil {
				log.Printf("WARN: Failed to save generated JWT secret to '%s': %v. Secret will only be in memory.", keyFilePath, err)
			} else {
				log.Printf("INFO: Successfully saved generated JWT secret to %s. Add this file to .gitignore!", keyFilePath)
			}
		}
	}

	// Ensure DbFilePath is absolute or relative to the current working directory
	absDbPath, err := filepath.Abs(cfg.DbFilePath)
	if err != nil {
		log.Printf("WARN: Could not determine absolute path for db-file '%s': %v. Using provided path.", cfg.DbFilePath, err)
	} else {
		cfg.DbFilePath = absDbPath
	}


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
	if envSecret := getEnv("JWT_SECRET", ""); envSecret != "" && envSecret == cfg.JwtSecret {
		return "Environment Variable (JWT_SECRET)"
	}
	// Check if it matches the content of the default generated key file
	defaultKeyBytes, err := os.ReadFile(defaultJwtKeyFile)
	if err == nil && string(defaultKeyBytes) == cfg.JwtSecret {
		return fmt.Sprintf("Generated File (%s)", defaultJwtKeyFile)
	}
	// If none of the above, it must have been generated in memory only
	return "Generated (In Memory)"
}

// Helper function to handle errors during config loading - could be expanded
func handleConfigError(field string, value string, err error, defaultValue any) {
    log.Printf("WARN: Invalid value for %s: '%s'. Using default %v. Error: %v", field, value, defaultValue, err)
}
