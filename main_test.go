package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMainBinary is the name of the compiled binary used for testing main.
const testMainBinary = "test_main_executable"

// buildMain builds the main package and returns the path to the executable
// and a cleanup function to remove it.
func buildMain(t *testing.T) (string, func()) {
	t.Helper()
	// Use a temporary directory for the build to avoid polluting the source dir
	// and potential conflicts if tests run in parallel in the future.
	// However, building in the current directory might be necessary if main
	// relies on relative paths for embedded assets or other files at runtime,
	// although using a temp dir is generally cleaner. Let's try current dir first.
	// tempDir := t.TempDir()
	// binaryPath := filepath.Join(tempDir, testMainBinary)
	binaryPath := testMainBinary // Build in current dir

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build main binary: %v\nOutput:\n%s", err, string(output))
	}

	cleanup := func() {
		err := os.Remove(binaryPath)
		if err != nil && !os.IsNotExist(err) {
			t.Logf("Warning: Failed to remove test binary %s: %v", binaryPath, err)
		}
	}

	// Ensure the binary path is absolute or relative to the working dir where tests run
	absPath, err := filepath.Abs(binaryPath)
	require.NoError(t, err, "Failed to get absolute path for test binary")

	return absPath, cleanup
}

// runMain runs the compiled main binary as a subprocess with given environment variables.
// It returns the exit code and the captured stderr output.
// It waits for a short duration for the process to potentially start and fail.
func runMain(t *testing.T, binaryPath string, envVars map[string]string) (exitCode int, stderr string) {
	t.Helper()

	cmd := exec.Command(binaryPath)

	// Set environment variables
	cmd.Env = os.Environ() // Inherit current environment
	for key, value := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	// Ensure required variables like PATH are present if not inherited fully
	// (os.Environ() usually handles this)

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	err := cmd.Start()
	require.NoError(t, err, "Failed to start main process")

	// Wait for the process to exit or timeout
	done := make(chan error, 1)
	go func() {
		waitErr := cmd.Wait()
		done <- waitErr
	}()

	select {
	case <-time.After(3 * time.Second): // Timeout for server start/fail
		// Process likely running (or hung), try to kill it
		_ = cmd.Process.Kill() // Use blank identifier, remove logging
		t.Logf("Main process timed out after 3 seconds, killing.") // Keep original timeout log
		return -1, stderrBuf.String() // Indicate timeout
	case err := <-done:
		stderr = stderrBuf.String()
		if err != nil {
			// Process exited with an error
			if exitError, ok := err.(*exec.ExitError); ok {
				return exitError.ExitCode(), stderr
			}
			// Other error (e.g., couldn't find binary - unlikely if Start succeeded)
			t.Fatalf("Main process failed with unexpected error: %v", err) // Revert Fatalf message
			return -1, stderr // Should not be reached
		}
		// Process exited successfully (code 0)
		return 0, stderr
	}
}

// TestMainFailureScenarios tests the main function's startup failure paths.
func TestMainFailureScenarios(t *testing.T) {
	binaryPath, cleanup := buildMain(t)
	defer cleanup()

	// --- Config Load Failure (Removed Missing JWT Secret Test) ---
	// The test for missing JWT secret causing failure is removed because
	// the application now generates a secret if none is provided.
	// We keep other config failure tests (e.g., invalid DB path).


	// --- Database Init Failure ---
	t.Run("DBInitFailure_InvalidPath", func(t *testing.T) {
		// Clean up potential default JWT key file
		_ = os.Remove("./docs.key")
		t.Cleanup(func() { _ = os.Remove("./docs.key") })

		// Create a directory where the DB file should be
		invalidDbPath := t.TempDir() // Use a directory instead of a file path

		env := map[string]string{
			"DOCSERVER_JWT_SECRET": "test-secret-for-db-fail-case", // Provide valid JWT
			"DOCSERVER_DB_FILE_PATH": invalidDbPath, // Point to the directory
		}

		exitCode, stderr := runMain(t, binaryPath, env)

		assert.NotEqual(t, 0, exitCode, "Expected non-zero exit code for DB init failure")
		// The error now occurs during config loading due to the path check
		assert.Contains(t, stderr, "CRITICAL: Failed to load configuration", "Stderr should contain config load error message")
		// Check for the specific reason: path is a directory
		assert.Contains(t, stderr, "points to a directory", "Stderr should mention the path is a directory")
	})

	// --- Server Bind Failure ---
	t.Run("ServerBindFailure_PortInUse", func(t *testing.T) {
		// Clean up potential default JWT key file
		_ = os.Remove("./docs.key")
		t.Cleanup(func() { _ = os.Remove("./docs.key") })

		// Find an available port first, then listen on it
		listener, err := net.Listen("tcp", ":0") // Listen on random available port
		require.NoError(t, err, "Failed to listen on a random port")
		addr := listener.Addr()
		// More robust way to get port: type assert to TCPAddr
		tcpAddr, ok := addr.(*net.TCPAddr)
		require.True(t, ok, "Listener address is not TCPAddr: %v", addr)
		port := fmt.Sprintf("%d", tcpAddr.Port) // Get port as string
		defer listener.Close() // Ensure listener is closed after test

		log.Printf("Dummy listener started on %s (port %s) for port conflict test", addr.String(), port)

		env := map[string]string{
			"DOCSERVER_JWT_SECRET": "test-secret-for-bind-fail-case",
			"DOCSERVER_LISTEN_PORT": port, // Tell main to use the port we are occupying
			// Use default DB path or a temp one
			"DOCSERVER_DB_FILE_PATH": filepath.Join(t.TempDir(), "test_bind_fail.json"),
		}

		exitCode, stderr := runMain(t, binaryPath, env)

		assert.NotEqual(t, 0, exitCode, "Expected non-zero exit code for server bind failure")
		assert.Contains(t, stderr, "CRITICAL: Server failed to start", "Stderr should contain server start error message")
		// Error message might vary slightly by OS ("address already in use", "bind: address already in use")
		assert.Contains(t, strings.ToLower(stderr), "address already in use", "Stderr should mention address in use")
	})
}