package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to reset flags and args for isolated tests
func resetFlagsAndArgs(args ...string) func() {
	originalArgs := os.Args
	os.Args = append([]string{"cmd"}, args...) // Prepend command name
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) // Reset default flag set

	return func() {
		os.Args = originalArgs // Restore original args
	}
}

// Helper to get absolute path for comparison, ignoring errors for simplicity in tests
func absPath(path string) string {
	abs, _ := filepath.Abs(path)
	return abs
}


func TestLoadConfig_Defaults(t *testing.T) {
	cleanup := resetFlagsAndArgs() // No args
	defer cleanup()

	// Ensure relevant env vars are unset
	os.Unsetenv("DOCSERVER_LISTEN_ADDRESS")
	os.Unsetenv("DOCSERVER_LISTEN_PORT")
	os.Unsetenv("DOCSERVER_DB_FILE_PATH")
	os.Unsetenv("DOCSERVER_SAVE_INTERVAL")
	os.Unsetenv("DOCSERVER_ENABLE_BACKUP")
	os.Unsetenv("DOCSERVER_JWT_SECRET_FILE")
	os.Unsetenv("DOCSERVER_JWT_SECRET")
	// Clean up potential generated key file before the test
	_ = os.Remove(defaultJwtKeyFile) // Ignore error if not found
	t.Cleanup(func() {
		_ = os.Remove(defaultJwtKeyFile) // Clean up after the test
	})

	// Provide a dummy JWT secret via env var for this test
	t.Setenv("DOCSERVER_JWT_SECRET", "test-default-secret")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, defaultAddress, cfg.ListenAddress)
	assert.Equal(t, defaultPort, cfg.ListenPort)
	assert.Equal(t, absPath(defaultDbFile), cfg.DbFilePath) // Compare absolute paths
	assert.Equal(t, defaultSaveInterval, cfg.SaveInterval)
	assert.Equal(t, defaultEnableBackup, cfg.EnableBackup)
	assert.Equal(t, defaultJwtSecretFile, cfg.JwtSecretFile) // Default is empty
	assert.Equal(t, defaultTokenLifetime, cfg.TokenLifetime)
	assert.Equal(t, defaultBcryptCost, cfg.BcryptCost)

	// Check JWT secret loading from env var (provided in test setup)
	assert.Equal(t, "test-default-secret", cfg.JwtSecret, "JWT Secret should be loaded from env var")
}


func TestLoadConfig_EnvVars(t *testing.T) {
	cleanup := resetFlagsAndArgs() // No args
	defer cleanup()

	// Set environment variables using the DOCSERVER_ prefix
	t.Setenv("DOCSERVER_LISTEN_ADDRESS", "192.168.1.100")
	t.Setenv("DOCSERVER_LISTEN_PORT", "9000")
	t.Setenv("DOCSERVER_DB_FILE_PATH", "/tmp/test_env.json")
	t.Setenv("DOCSERVER_SAVE_INTERVAL", "15s")
	t.Setenv("DOCSERVER_ENABLE_BACKUP", "false")
	t.Setenv("DOCSERVER_JWT_SECRET_FILE", "/etc/secrets/jwt_env.key") // File doesn't exist, will fallback
	t.Setenv("DOCSERVER_JWT_SECRET", "env_secret_key_longer_than_32_bytes") // This should be used as fallback

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.100", cfg.ListenAddress)
	assert.Equal(t, "9000", cfg.ListenPort)
	assert.Equal(t, absPath("/tmp/test_env.json"), cfg.DbFilePath)
	assert.Equal(t, 15*time.Second, cfg.SaveInterval)
	assert.Equal(t, false, cfg.EnableBackup)
	assert.Equal(t, "/etc/secrets/jwt_env.key", cfg.JwtSecretFile)

	// JWT Secret: Since JWT_SECRET_FILE is set via env, it should try to load from that file.
	// As the file likely doesn't exist, it should fall back to JWT_SECRET env var.
	assert.Equal(t, "env_secret_key_longer_than_32_bytes", cfg.JwtSecret)

}

func TestLoadConfig_Flags(t *testing.T) {
	// Define expected values different from defaults and potential env vars
	expectedAddr := "127.0.0.1"
	expectedPort := "8888"
	expectedDbFile := "./flag_db.json"
	expectedIntervalStr := "2m"
	expectedIntervalDur := 2 * time.Minute
	expectedBackup := "false" // String because flags parse bools differently
	expectedJwtFile := "flag_jwt.key"

	cleanup := resetFlagsAndArgs(
		"--address", expectedAddr,
		"--port", expectedPort,
		"--db-file", expectedDbFile,
		"--save-interval", expectedIntervalStr,
		"--enable-backup="+expectedBackup, // Use name=value format for bools
		"--jwt-secret-file", expectedJwtFile,
	)
	defer cleanup()

	// Ensure env vars are unset to isolate flag testing
	os.Unsetenv("DOCSERVER_LISTEN_ADDRESS")
	os.Unsetenv("DOCSERVER_LISTEN_PORT")
	os.Unsetenv("DOCSERVER_DB_FILE_PATH")
	os.Unsetenv("DOCSERVER_SAVE_INTERVAL")
	os.Unsetenv("DOCSERVER_ENABLE_BACKUP")
	os.Unsetenv("DOCSERVER_JWT_SECRET_FILE")
	os.Unsetenv("DOCSERVER_JWT_SECRET")
	// Provide a dummy JWT secret via env var. Even though generation is now possible,
	// this test specifically checks flag precedence where the flag file doesn't exist,
	// so we want it to fall back to the env var, not generation.
	t.Setenv("DOCSERVER_JWT_SECRET", "test-flag-secret-fallback")
	// Clean up potential generated key file before/after test
	_ = os.Remove(defaultJwtKeyFile)
	t.Cleanup(func() { _ = os.Remove(defaultJwtKeyFile) })


	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, expectedAddr, cfg.ListenAddress)
	assert.Equal(t, expectedPort, cfg.ListenPort)
	assert.Equal(t, absPath(expectedDbFile), cfg.DbFilePath)
	assert.Equal(t, expectedIntervalDur, cfg.SaveInterval)
	assert.Equal(t, false, cfg.EnableBackup) // Parsed boolean
	assert.Equal(t, expectedJwtFile, cfg.JwtSecretFile)

	// JWT Secret: File is specified by flag, but doesn't exist.
	// It should fall back to the JWT_SECRET env var provided in the test setup.
	assert.Equal(t, "test-flag-secret-fallback", cfg.JwtSecret, "JWT Secret should fall back to env var when flag file doesn't exist")
}


func TestLoadConfig_Precedence(t *testing.T) {
	// Flag > Env > Default
	// Test with PORT variable

	// Default: 8080
	// Env: 9000
	// Flag: 9999

	expectedPort := "9999" // Flag value

	t.Setenv("DOCSERVER_LISTEN_PORT", "9000") // Set Env var

	cleanup := resetFlagsAndArgs("--port", expectedPort) // Set flag
	defer cleanup()
	// Provide a dummy JWT secret to avoid generation path
	t.Setenv("DOCSERVER_JWT_SECRET", "test-precedence-secret")
	// Clean up potential generated key file before/after test
	_ = os.Remove(defaultJwtKeyFile)
	t.Cleanup(func() { _ = os.Remove(defaultJwtKeyFile) })

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, expectedPort, cfg.ListenPort, "Flag value should take precedence")
}


func TestLoadConfig_SaveIntervalParsing(t *testing.T) {
	// Provide a dummy JWT secret for all sub-tests
	t.Setenv("DOCSERVER_JWT_SECRET", "test-interval-secret") // Avoid generation path
	// Clean up potential generated key file before/after test
	_ = os.Remove(defaultJwtKeyFile)
	t.Cleanup(func() { _ = os.Remove(defaultJwtKeyFile) })
	t.Run("Valid Duration Flag", func(t *testing.T) {
		cleanup := resetFlagsAndArgs("--save-interval", "5m30s")
		defer cleanup()
		os.Unsetenv("DOCSERVER_SAVE_INTERVAL")

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, 5*time.Minute+30*time.Second, cfg.SaveInterval)
	})

	t.Run("Invalid Duration Flag", func(t *testing.T) {
		cleanup := resetFlagsAndArgs("--save-interval", "invalid")
		defer cleanup()
		os.Unsetenv("DOCSERVER_SAVE_INTERVAL")

		// LoadConfig logs a warning but uses default
		cfg, err := LoadConfig()
		require.NoError(t, err) // LoadConfig itself doesn't return error for this
		assert.Equal(t, defaultSaveInterval, cfg.SaveInterval, "Should fall back to default on invalid duration")
	})

	t.Run("Valid Duration Env", func(t *testing.T) {
		cleanup := resetFlagsAndArgs() // No flag
		defer cleanup()
		t.Setenv("DOCSERVER_SAVE_INTERVAL", "1h")

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, 1*time.Hour, cfg.SaveInterval)
	})
}

func TestLoadConfig_EnableBackupParsing(t *testing.T) {
	// Provide a dummy JWT secret for all sub-tests
	t.Setenv("DOCSERVER_JWT_SECRET", "test-backup-secret") // Avoid generation path
	// Clean up potential generated key file before/after test
	_ = os.Remove(defaultJwtKeyFile)
	t.Cleanup(func() { _ = os.Remove(defaultJwtKeyFile) })
	testCases := []struct {
		name          string
		envValue      *string // Pointer to distinguish between unset and empty string
		flagValue     *string
		expectedBool  bool
	}{
		{name: "Default", envValue: nil, flagValue: nil, expectedBool: defaultEnableBackup},

		// Env Var variations (case-insensitive)
		{name: "Env true", envValue: ptr("true"), flagValue: nil, expectedBool: true},
		{name: "Env TRUE", envValue: ptr("TRUE"), flagValue: nil, expectedBool: true},
		{name: "Env 1", envValue: ptr("1"), flagValue: nil, expectedBool: true},
		{name: "Env yes", envValue: ptr("yes"), flagValue: nil, expectedBool: true},
		{name: "Env false", envValue: ptr("false"), flagValue: nil, expectedBool: false},
		{name: "Env FALSE", envValue: ptr("FALSE"), flagValue: nil, expectedBool: false},
		{name: "Env 0", envValue: ptr("0"), flagValue: nil, expectedBool: false},
		{name: "Env no", envValue: ptr("no"), flagValue: nil, expectedBool: false},
		{name: "Env invalid (fallback)", envValue: ptr("invalid"), flagValue: nil, expectedBool: defaultEnableBackup},

		// Flag variations (overrides env)
		{name: "Flag true", envValue: ptr("false"), flagValue: ptr("true"), expectedBool: true}, // Flag overrides env
		{name: "Flag false", envValue: ptr("true"), flagValue: ptr("false"), expectedBool: false},// Flag overrides env
		{name: "Flag 1 (parsed as true)", envValue: nil, flagValue: ptr("1"), expectedBool: true}, // flag package parses "1" as true
		{name: "Flag 0 (parsed as false)", envValue: nil, flagValue: ptr("0"), expectedBool: false},// flag package parses "0" as true
		// Note: flag package bool parsing is stricter than getEnvBool (e.g., doesn't accept "yes")

	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{}
			if tc.flagValue != nil {
				// Bool flags can be tricky, use --name=value format
				args = append(args, "--enable-backup="+*tc.flagValue)
			}
			cleanup := resetFlagsAndArgs(args...)
			defer cleanup()

			if tc.envValue != nil {
				t.Setenv("DOCSERVER_ENABLE_BACKUP", *tc.envValue)
			} else {
				os.Unsetenv("DOCSERVER_ENABLE_BACKUP")
			}

			cfg, err := LoadConfig()
			require.NoError(t, err)
			assert.Equal(t, tc.expectedBool, cfg.EnableBackup)
		})
	}
}

// Helper function to return pointer to string
func ptr(s string) *string {
	return &s
}


// --- JWT Secret Loading/Generation Tests ---

// Helper to create a temporary file with content
func createTempFile(t *testing.T, content string) string {
	file, err := os.CreateTemp("", "config_test_jwt_")
	require.NoError(t, err, "Failed to create temp file")
	_, err = file.WriteString(content)
	require.NoError(t, err, "Failed to write to temp file")
	err = file.Close()
	require.NoError(t, err, "Failed to close temp file")
	return file.Name()
}

func TestLoadConfig_JWTSecretHandling(t *testing.T) {
	// General cleanup for the default key file for all sub-tests
	t.Cleanup(func() {
		_ = os.Remove(defaultJwtKeyFile)
	})

	// --- Test Case 1: Secret from File (via Flag) ---
	t.Run("SecretFromFileFlag", func(t *testing.T) {
		secretContent := "secret_from_flag_file_content_very_secure"
		tempFile := createTempFile(t, secretContent)
		defer os.Remove(tempFile)

		cleanup := resetFlagsAndArgs("--jwt-secret-file", tempFile)
		defer cleanup()
		os.Unsetenv("DOCSERVER_JWT_SECRET_FILE") // Ensure flag takes precedence over potential env var for file path
		os.Unsetenv("DOCSERVER_JWT_SECRET")      // Ensure env secret is not used
		_ = os.Remove(defaultJwtKeyFile)         // Ensure default key file doesn't interfere

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, secretContent, cfg.JwtSecret, "JWT Secret should match file content")
		assert.Equal(t, tempFile, cfg.JwtSecretFile, "JwtSecretFile path should match flag")
	})

	// --- Test Case 2: Secret from File (via Env) ---
	t.Run("SecretFromFileEnv", func(t *testing.T) {
		secretContent := "secret_from_env_file_content_also_secure"
		tempFile := createTempFile(t, secretContent)
		defer os.Remove(tempFile)

		cleanup := resetFlagsAndArgs() // No flag
		defer cleanup()
		t.Setenv("DOCSERVER_JWT_SECRET_FILE", tempFile) // Set file path via env
		os.Unsetenv("DOCSERVER_JWT_SECRET")             // Ensure env secret is not used
		_ = os.Remove(defaultJwtKeyFile)                // Ensure default key file doesn't interfere

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, secretContent, cfg.JwtSecret, "JWT Secret should match file content")
		assert.Equal(t, tempFile, cfg.JwtSecretFile, "JwtSecretFile path should match env var")
	})

	// --- Test Case 3: Secret from Env Var (No File Specified) ---
	t.Run("SecretFromEnvVar", func(t *testing.T) {
		envSecret := "environment_variable_secret_shhh"
		cleanup := resetFlagsAndArgs() // No flag
		defer cleanup()
		os.Unsetenv("DOCSERVER_JWT_SECRET_FILE")        // Ensure no file path is set
		t.Setenv("DOCSERVER_JWT_SECRET", envSecret)     // Set secret via env var
		_ = os.Remove(defaultJwtKeyFile)                // Ensure default key file doesn't interfere

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, envSecret, cfg.JwtSecret, "JWT Secret should match env var")
		assert.Empty(t, cfg.JwtSecretFile, "JwtSecretFile path should be empty")
	})

	// --- Test Case 4: Secret from Env Var (File Specified but Not Found) ---
	t.Run("SecretFromEnvVarFallback", func(t *testing.T) {
		envSecret := "fallback_environment_secret"
		nonExistentFile := filepath.Join(t.TempDir(), "non_existent.key") // Use t.TempDir for auto cleanup

		cleanup := resetFlagsAndArgs("--jwt-secret-file", nonExistentFile) // Flag points to non-existent file
		defer cleanup()
		os.Unsetenv("DOCSERVER_JWT_SECRET_FILE")    // Unset env var for file path
		t.Setenv("DOCSERVER_JWT_SECRET", envSecret) // Set env secret as fallback
		_ = os.Remove(defaultJwtKeyFile)            // Ensure default key file doesn't interfere

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, envSecret, cfg.JwtSecret, "JWT Secret should fall back to env var")
		assert.Equal(t, nonExistentFile, cfg.JwtSecretFile, "JwtSecretFile path should still be set from flag")
	})

	// --- Test Case 5: Secret from Default Key File ---
	t.Run("SecretFromDefaultKeyFile", func(t *testing.T) {
		defaultKeyContent := "secret_from_default_dot_key_file"
		err := os.WriteFile(defaultJwtKeyFile, []byte(defaultKeyContent), 0600)
		require.NoError(t, err, "Failed to create default key file")
		// Cleanup handled by top-level t.Cleanup

		cleanup := resetFlagsAndArgs() // No flag
		defer cleanup()
		os.Unsetenv("DOCSERVER_JWT_SECRET_FILE") // Ensure no specific file path
		os.Unsetenv("DOCSERVER_JWT_SECRET")      // Ensure no env secret

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, defaultKeyContent, cfg.JwtSecret, "JWT Secret should match default key file content")
		assert.Empty(t, cfg.JwtSecretFile, "JwtSecretFile path should be empty when using default")
	})

	// --- Test Case 6: Generated Secret (No File, No Env Var, No Default Key File) ---
	t.Run("GeneratedSecret", func(t *testing.T) {
		cleanup := resetFlagsAndArgs() // No flag
		defer cleanup()
		os.Unsetenv("DOCSERVER_JWT_SECRET_FILE") // Ensure no file path
		os.Unsetenv("DOCSERVER_JWT_SECRET")      // Ensure no env secret
		_ = os.Remove(defaultJwtKeyFile)         // Ensure default key file does not exist

		cfg, err := LoadConfig()
		require.NoError(t, err, "LoadConfig should succeed by generating a secret")
		assert.NotEmpty(t, cfg.JwtSecret, "Generated JWT Secret should not be empty")
		assert.Len(t, cfg.JwtSecret, 64, "Generated JWT Secret should be 64 hex characters (32 bytes)") // Default generation length
		assert.Empty(t, cfg.JwtSecretFile, "JwtSecretFile path should be empty when generating")

		// Verify the key was saved to the default file
		savedBytes, err := os.ReadFile(defaultJwtKeyFile)
		require.NoError(t, err, "Failed to read generated default key file")
		assert.Equal(t, cfg.JwtSecret, string(savedBytes), "Saved key file content should match generated secret")
	})

	// --- Test Case 7: Generated Secret (Specified File Not Found, No Env Var, No Default Key File) ---
	// Even if a specific file is requested but fails, generation should still occur if no other source exists.
	t.Run("GeneratedSecretWithFailedFile", func(t *testing.T) {
		nonExistentFile := filepath.Join(t.TempDir(), "non_existent_gen.key")

		cleanup := resetFlagsAndArgs("--jwt-secret-file", nonExistentFile) // Flag points to non-existent file
		defer cleanup()
		os.Unsetenv("DOCSERVER_JWT_SECRET_FILE") // Unset env var for file path
		os.Unsetenv("DOCSERVER_JWT_SECRET")      // Unset env secret
		_ = os.Remove(defaultJwtKeyFile)         // Ensure default key file does not exist

		cfg, err := LoadConfig()
		require.NoError(t, err, "LoadConfig should succeed by generating a secret even if specified file failed")
		assert.NotEmpty(t, cfg.JwtSecret, "Generated JWT Secret should not be empty")
		assert.Len(t, cfg.JwtSecret, 64, "Generated JWT Secret should be 64 hex characters")
		assert.Equal(t, nonExistentFile, cfg.JwtSecretFile, "JwtSecretFile path should still reflect the requested (but failed) file")

		// Verify the key was saved to the default file
		savedBytes, err := os.ReadFile(defaultJwtKeyFile)
		require.NoError(t, err, "Failed to read generated default key file")
		assert.Equal(t, cfg.JwtSecret, string(savedBytes), "Saved key file content should match generated secret")
	})
}


// --- DbFilePath Absolute Path Tests ---

func TestLoadConfig_DbFilePathAbsolute(t *testing.T) {
	// Provide a dummy JWT secret for all sub-tests
	t.Setenv("DOCSERVER_JWT_SECRET", "test-dbpath-secret") // Avoid generation path
	// Clean up potential generated key file before/after test
	_ = os.Remove(defaultJwtKeyFile)
	t.Cleanup(func() { _ = os.Remove(defaultJwtKeyFile) })
	originalWd, err := os.Getwd()
	require.NoError(t, err, "Failed to get current working directory")

	// Create a temporary directory to act as a different working directory
	tempWd := t.TempDir()

	testCases := []struct {
		name         string
		dbFileArg    string // Value passed via flag/env
		expectedPath string // Expected absolute path relative to originalWd
	}{
		{name: "Relative path", dbFileArg: "relative/db.json", expectedPath: filepath.Join(originalWd, "relative/db.json")},
		{name: "Current dir path", dbFileArg: "./current_db.json", expectedPath: filepath.Join(originalWd, "current_db.json")},
		{name: "Absolute path", dbFileArg: "/tmp/absolute_db.json", expectedPath: "/tmp/absolute_db.json"}, // Should remain absolute
		{name: "Default path", dbFileArg: "", expectedPath: filepath.Join(originalWd, defaultDbFile)}, // Default is relative to WD
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Change working directory for the test
			err := os.Chdir(tempWd)
			require.NoError(t, err, "Failed to change working directory")
			// Ensure we change back after the test
			defer func() {
				err := os.Chdir(originalWd)
				if err != nil {
					t.Logf("Warning: Failed to change back to original working directory: %v", err)
				}
			}()


			args := []string{}
			if tc.dbFileArg != "" {
				args = append(args, "--db-file", tc.dbFileArg)
			}
			cleanup := resetFlagsAndArgs(args...)
			defer cleanup()
			os.Unsetenv("DOCSERVER_DB_FILE_PATH") // Isolate flag behaviour


			// Load config *while in the temporary working directory*
			cfg, err := LoadConfig()
			require.NoError(t, err)

			// The DbFilePath in cfg should be absolute, relative to the *original* WD where LoadConfig was called from
			// (because resetFlagsAndArgs sets os.Args[0] which flag uses implicitly, and Abs uses WD)
			// Let's re-evaluate this assumption. filepath.Abs uses the *current* working directory.
			// So, the expected path should be relative to tempWd if the input was relative.

			var expectedAbsPath string
			if filepath.IsAbs(tc.dbFileArg) {
				expectedAbsPath = tc.dbFileArg // Absolute paths stay absolute
			} else if tc.dbFileArg == "" {
				// Default path is relative to the WD where LoadConfig runs
				expectedAbsPath = filepath.Join(tempWd, defaultDbFile)
			} else {
				// Relative path is relative to the WD where LoadConfig runs
				expectedAbsPath = filepath.Join(tempWd, tc.dbFileArg)
			}


			assert.Equal(t, expectedAbsPath, cfg.DbFilePath, "Absolute DbFilePath mismatch")
			assert.True(t, filepath.IsAbs(cfg.DbFilePath), "DbFilePath should be absolute")
		})
	}
}

// --- handleConfigError Test ---

// TestHandleConfigError checks if the helper function runs without panicking.
// Testing the actual log output is often brittle and might require more complex setup.
func TestHandleConfigError(t *testing.T) {
	// Simply call the function with dummy data to ensure it executes.
	// We are not capturing log output here.
	assert.NotPanics(t, func() {
		handleConfigError("testField", "badValue", assert.AnError, "defaultValue")
	}, "handleConfigError should not panic")
}