# Plan: Go Gin REST API for Note Server

This document outlines the plan to reimplement the Note Server API using the Go Gin framework, incorporating a RESTful design, a custom single-file JSON database, advanced querying capabilities, JWT authentication, and robust configuration.

## 1. Core Concepts & Data Models

We'll define three main resources: Profiles, Documents, and ShareRecords. Passwords will be hashed using bcrypt.

**File:** `db/models.go`

```go
package models

import (
	"sync"
	"time"
)

// Profile represents a user account
type Profile struct {
	ID             string    `json:"id"`              // Unique ID (UUID, dashless)
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Email          string    `json:"email"`           // Unique, used for login
	PasswordHash   string    `json:"-"`               // Store hash, exclude from JSON responses.
	CreationDate   time.Time `json:"creation_date"`   // UTC
	LastModifiedDate time.Time `json:"last_modified_date"` // UTC
	Extra          any       `json:"extra,omitempty"` // User-defined data
}

// Document represents a stored document
type Document struct {
	ID             string    `json:"id"`              // Unique ID (UUID, dashless)
	OwnerID        string    `json:"owner_id"`        // Profile ID of the owner
	Content        any       `json:"content"`         // Can be any JSON structure or simple text
	CreationDate   time.Time `json:"creation_date"`   // UTC
	LastModifiedDate time.Time `json:"last_modified_date"` // UTC
}

// ShareRecord links a document to users it's shared with
// There will be one ShareRecord per Document ID that has shares.
type ShareRecord struct {
	DocumentID string   `json:"-"`           // Document ID (acts as the key in the map, dashless)
	SharedWith []string `json:"shared_with"` // List of Profile IDs allowed access (dashless)
}

// Database holds all application data and manages concurrent access
type Database struct {
	Profiles     map[string]Profile     `json:"profiles"`      // Keyed by Profile ID (dashless)
	Documents    map[string]Document    `json:"documents"`     // Keyed by Document ID (dashless)
	ShareRecords map[string]ShareRecord `json:"share_records"` // Keyed by Document ID (dashless)

	// Mutex for thread-safe access to the maps
	mu sync.RWMutex `json:"-"` // Exclude mutex from serialization

    // File path for persistence (obtained from configuration)
    filePath string `json:"-"`
    // Backup enabled flag (obtained from configuration)
    backupEnabled bool `json:"-"`
    // Save interval (obtained from configuration)
    saveInterval time.Duration `json:"-"`
}

// Add methods to Database for Load, Save, and CRUD operations on Profiles, Documents, ShareRecords, ensuring mutex usage.
```

## 2. Configuration

The server's behavior can be configured via command-line arguments and environment variables. Command-line arguments take precedence over environment variables, which take precedence over default values.

| Feature          | CLI Argument             | Environment Variable | Default Value        | Description                                                                                                                            |
| :--------------- | :----------------------- | :------------------- | :------------------- | :------------------------------------------------------------------------------------------------------------------------------------- |
| **Server**       |                          |                      |                      |                                                                                                                                        |
| Listen Address   | `--address <ip>`         | `ADDRESS`            | `0.0.0.0`            | IP address for the server to listen on.                                                                                                |
| Listen Port      | `--port <number>`        | `PORT`               | `8080`               | Port number for the server to listen on.                                                                                               |
| **Database**     |                          |                      |                      |                                                                                                                                        |
| File Path        | `--db-file <path>`       | `DB_FILE`            | `./docs.json`        | Path to the JSON database file.                                                                                                        |
| Save Interval    | `--save-interval <dur>`  | `SAVE_INTERVAL`      | `3s`                 | Debounce interval for saving (e.g., `5s`, `100ms`). `0s` or less means instant save.                                                   |
| Enable Backup    | `--enable-backup <bool>` | `ENABLE_BACKUP`      | `true`               | Create a `.bak` file before saving changes (`true`/`false`).                                                                           |
| **Authentication** |                          |                      |                      |                                                                                                                                        |
| JWT Secret File  | `--jwt-secret-file <path>`| `JWT_SECRET_FILE`    | *(None)*             | Path to a file containing the JWT secret key. Overrides `JWT_SECRET`.                                                                  |
| JWT Secret       | *(N/A)*                  | `JWT_SECRET`         | *(None)*             | JWT secret key string. Used if file path is not provided.                                                                              |
| *JWT Key Gen*    | *(N/A)*                  | *(N/A)*              | *(Generated)*        | *If neither file nor ENV var is set, a key is generated and saved to `./docs.key`.*                                                    |
| Token Lifetime   | *(N/A)*                  | *(N/A)*              | `1h`                 | Default lifetime for JWT access tokens. (Not currently configurable via arg/env).                                                        |
| Bcrypt Cost      | *(N/A)*                  | *(N/A)*              | `12`                 | Cost factor for bcrypt password hashing. (Not currently configurable via arg/env).                                                       |

**Implementation Notes:**
*   Use a library like `spf13/pflag` and `spf13/viper` or standard `flag` and `os.Getenv` to handle configuration loading and precedence.
*   Parsing logic belongs in `main.go` or a dedicated `config` package.
*   The loaded configuration values should be passed to or made available to the relevant components (database, auth utils, server setup).
*   Ensure `./docs.key` is added to `.gitignore`.

## 3. REST API Endpoint Design

Map functionalities to standard REST endpoints.

| Feature             | Original Method(s)                               | Proposed REST Endpoint                 | HTTP Method | Auth Required | Notes                                                                                                |
| :------------------ | :----------------------------------------------- | :------------------------------------- | :---------- | :------------ | :--------------------------------------------------------------------------------------------------- |
| **Authentication**  |                                                  |                                        |             |               |                                                                                                      |
| Sign Up             | `createAccount`, `registerAccount`               | `/auth/signup`                         | POST        | No            | Body: {email, password, first\_name, last\_name, extra?}. Returns Profile (no hash).                  |
| Login               | `authenticate`                                   | `/auth/login`                          | POST        | No            | Body: {email, password}. Returns JWT.                                                                |
| Logout              | `signOut`                                        | `/auth/logout`                         | POST        | Yes (JWT)     | Client-side token deletion. Returns 204 No Content.                                                  |
| Forgot Password     | `forgotPassword`                                 | `/auth/forgot-password`                | POST        | No            | Body: {email}. Generates OTP, stores temporarily, prints to console. Returns 202 Accepted.           |
| Reset Password      | `registerAccount` (with temp password)           | `/auth/reset-password`                 | POST        | No            | Body: {email, otp, new_password}. Verifies OTP, updates password. Returns 204 No Content.            |
| **Profiles**        |                                                  |                                        |             |               |                                                                                                      |
| Get Current Profile | (Implicit via token)                             | `/profiles/me`                         | GET         | Yes (JWT)     | Returns the profile of the authenticated user.                                                       |
| Update Profile      | `setAccount`                                     | `/profiles/me`                         | PUT/PATCH   | Yes (JWT)     | Updates the authenticated user's profile (first/last name, extra).                                   |
| Delete Profile      | `deleteAccount`                                  | `/profiles/me`                         | DELETE      | Yes (JWT)     | Deletes the authenticated user's account and associated data. Returns 204 No Content.                |
| Search Profiles     | `getAllAccounts` (modified)                      | `/profiles`                            | GET         | Yes (JWT)     | Query params: `email`, `first_name`, `last_name`. Pagination (`page`, `limit`).                      |
| **Documents**       |                                                  |                                        |             |               |                                                                                                      |
| Create Document     | `setDocument` (if ID doesn't exist)              | `/documents`                           | POST        | Yes (JWT)     | Body: {content}. Returns created Document. OwnerID set from JWT.                                     |
| Get Documents       | `getDocuments`, `getDocument` (modified)         | `/documents`                           | GET         | Yes (JWT)     | **Returns ONLY documents owned by or shared with the authenticated user.** Query params filter *within* this set: `scope` (owned, shared, all - default all), `content_query` (JSON path query array), `sort_by` (creation\_date, last\_modified\_date), `order` (asc, desc), `page`, `limit`. |
| Get Document by ID  | `getDocument`                                    | `/documents/{id}`                      | GET         | Yes (JWT)     | Returns a specific document if owned or shared. ID in path is dashless UUID.                         |
| Update Document     | `setDocument` (if ID exists)                     | `/documents/{id}`                      | PUT/PATCH   | Yes (JWT)     | Updates document content if owned. ID in path is dashless UUID. PUT replaces content, PATCH could merge. |
| Delete Document     | `deleteDocument`                                 | `/documents/{id}`                      | DELETE      | Yes (JWT)     | Deletes document if owned. ID in path is dashless UUID. Also deletes associated ShareRecord. Returns 204. |
| **Sharing**         |                                                  |                                        |             |               |                                                                                                      |
| Get Sharers         | `getDocumentAccessors`                           | `/documents/{id}/shares`               | GET         | Yes (JWT)     | Returns list of Profile IDs (dashless) the document is shared with (owner only). ID in path is dashless UUID. |
| Set/Update Sharers  | `setDocumentAccessors`                           | `/documents/{id}/shares`               | PUT         | Yes (JWT)     | Replaces the list of Profile IDs (owner only). Body: `{"shared_with": ["id1", "id2"]}` (dashless IDs). ID in path is dashless UUID. Returns 204. |
| Add Sharer          | (Derived from `setDocumentAccessors`)            | `/documents/{id}/shares/{profile_id}`  | PUT         | Yes (JWT)     | Adds a specific profile ID (dashless) to the share list (owner only). IDs in path are dashless UUIDs. Returns 204. |
| Remove Sharer       | (Derived from `setDocumentAccessors`)            | `/documents/{id}/shares/{profile_id}`  | DELETE      | Yes (JWT)     | Removes a specific profile ID (dashless) from the share list (owner only). IDs in path are dashless UUIDs. Returns 204. |

## 4. JSON Database Implementation

*   **Storage File:** A single JSON file storing the marshalled `Database` struct. Path determined by Configuration.
*   **Location:** Core logic in `db/database.go`.
*   **Loading:**
    *   On server start, attempt to read the configured database file (path from Configuration).
    *   Unmarshal JSON into the in-memory `Database` struct.
    *   **Error Handling:**
        *   If the file doesn't exist, initialize with empty maps and log this event.
        *   If JSON unmarshalling fails, log the error critically and **stop server startup**.
        *   Log other file read errors but potentially continue with an empty database if recoverable.
*   **Saving Strategy (Debounced Periodic / Instant):**
    *   Save interval determined by Configuration.
    *   **Debouncing:** Use a mechanism (e.g., a timer reset on each change request) to ensure that saves only happen after a period of inactivity defined by the save interval.
    *   **Trigger:** A `requestSave()` method will be called after *every* successful write operation (Create, Update, Delete). This method triggers/resets the debounce timer.
    *   **Instant Save:** If save interval from Configuration is `0s` or less, `requestSave()` triggers an immediate save.
    *   **Actual Save Process (`persist()`):**
        1.  Acquire a read lock (`db.mu.RLock()`) to safely marshal the current state. *Note: A full write lock (`db.mu.Lock()`) might be needed depending on implementation details.*
        2.  Marshal the *entire* in-memory `Database` struct to JSON.
        3.  **Atomic Write:**
            *   Write the marshalled JSON to a temporary file (e.g., `docs.json.tmp`) in the same directory as the configured DB file.
            *   If backup is enabled (via Configuration), rename the current DB file to `.bak`. Handle potential errors if `.bak` already exists.
            *   Atomically rename the temporary file to the final database file path (from Configuration).
        4.  Release the lock (`db.mu.RUnlock()` or `db.mu.Unlock()`).
    *   **Error Handling:**
        *   Log any errors during marshalling, file writing, or renaming.
        *   If saving fails, keep the data in memory and schedule a retry shortly. Log retry attempts.
*   **Backup:** Controlled by Configuration.
*   **Thread Safety:** Use `sync.RWMutex` within the `Database` struct.
    *   `RLock()`/`RUnlock()` for read operations.
    *   `Lock()`/`Unlock()` for in-memory write operations.

## 5. Query Engine Design

*   **Location:** `db/query.go` (or methods on `Database` struct).
*   **Document Content Query (`content_query`):**
    *   Accepts an array of strings representing conditions and logical operators: `["path operator value", "logic", "path operator value", ...]`. Logic operators must be `and` or `or`.
    *   **Parsing:** Parse each condition string (`path operator value`).
    *   **Path Access:** Use a library like `tidwall/gjson` to access nested JSON `Content` via dot notation (`key1.key2`). If the path is empty or refers to the root, apply operators to the entire `Content`.
    *   **Operators:**
        *   **General:** `equals`, `notEquals`
        *   **Numeric/Comparable:** `greaterThan`, `lessThan`, `greaterThanOrEquals`, `lessThanOrEquals`
        *   **String/Array:** `contains` (checks substring for strings, element existence for arrays)
        *   **String Only:** `startsWith`, `endsWith`
    *   **Case Insensitivity:** Append `-insensitive` suffix to string operators for case-insensitive matching (e.g., `equals-insensitive`, `contains-insensitive`, `startsWith-insensitive`, `endsWith-insensitive`).
    *   **Plain Text Content:** If `Content` is not valid JSON (e.g., a plain string), only the following operators are valid (applied to the entire string content): `equals`, `notEquals`, `contains`, `startsWith`, `endsWith` (and their `-insensitive` variants). Using other operators on non-JSON content results in an error.
    *   **Type Handling:** Correctly handle comparisons for strings, numbers (integers/floats), booleans, and null values. Type mismatches during comparison result in an error (e.g., comparing a string with `greaterThan`).
    *   **Array `contains`:** The syntax `path contains value` checks if the array at `path` includes the specified `value` (string, number, boolean, or null).
    *   **Evaluation:** Evaluate the parsed query conditions and logic against each document's `Content`.
    *   **Error Handling:** Return HTTP 400 (Bad Request) with a descriptive message for malformed queries (invalid syntax, unknown operators, invalid logic operators, non-existent paths during evaluation, type mismatches during comparison).
*   **Filtering (`GET /documents`):**
    1.  Apply `content_query` if present.
    2.  Apply `scope` filtering based on the authenticated user's ID (`OwnerID` and `ShareRecords`).
*   **Sorting (`GET /documents`):** Apply `sort_by` (creation\_date, last\_modified\_date) and `order` (asc, desc) using `sort.Slice` on the filtered results.
*   **Pagination (`GET /documents`, `GET /profiles`):** Apply `page` and `limit` (max 100) to the final sorted list before returning. Calculate slice indices.
*   **Profile Querying (`GET /profiles`):** Filter by `email`, `first_name`, `last_name` (case-insensitive contains/exact match).

## 6. Authentication (JWT)

*   **Library:** `golang-jwt/jwt/v5`.
*   **Location:** `utils/auth.go` (helpers), `api/middleware.go` (Gin middleware).
*   **JWT Secret Key:** Determined by Configuration (CLI arg, ENV var, or generated file). **MUST NOT be hardcoded.**
*   **Middleware:** Gin middleware for protected routes. Extracts Bearer token from `Authorization` header, validates signature and expiry using the configured secret key. Adds `userID` (dashless UUID) to Gin context (`c.Set("userID", ...)`) on success, returns `401 Unauthorized` on failure.
*   **Claims:** `user_id` (dashless UUID), `email`, `exp` (expiration), `iat` (issued at), `iss` (issuer, e.g., "docserver").
*   **Token Lifetime:** Default 1 hour (`exp` claim). See Configuration section.
*   **Token Refresh:** Not implemented. Users must re-authenticate via `/auth/login` to get a new token.
*   **Logout:** Client-side responsibility to discard the token. No server-side blocklist.
*   **Password Hashing:**
    *   Use `golang.org/x/crypto/bcrypt` in `utils/auth.go`.
    *   Use default cost factor of 12. See Configuration section.
    *   Hash on signup and password reset. Compare hash on login.
*   **Password Reset (OTP):**
    *   `/auth/forgot-password`:
        *   Generate a secure One-Time Password (OTP, e.g., 6-digit number).
        *   Store the OTP temporarily server-side (e.g., in-memory map `map[email]otpRecord{otp string, expiry time.Time}`) with a short expiry (e.g., 5 minutes). Use a mutex for safe concurrent access to this map.
        *   **Print the OTP clearly to the server console** (e.g., using colored text) for the user to manually retrieve (simulating email/SMS delivery for this academic context).
    *   `/auth/reset-password`:
        *   Requires `email`, `otp`, `new_password`.
        *   Verify the provided `otp` against the stored OTP for the `email`. Check expiry.
        *   If valid, hash the `new_password` and update the user's `PasswordHash` in the database.
        *   Delete the used OTP from the temporary store.
        *   Return 400/401 for invalid/expired OTP.

## 7. Project Structure

```
/docserver                 # Project root
├── go.mod
├── go.sum
├── main.go                 # Entry point, Gin setup, routes, config parsing, logging setup
├── api/                    # Gin handlers & middleware
│   ├── handlers_auth.go    # Auth related handlers
│   ├── handlers_docs.go    # Document related handlers
│   ├── handlers_profiles.go# Profile related handlers
│   ├── handlers_shares.go  # Sharing related handlers
│   └── middleware.go       # Auth middleware
├── db/                     # Database logic & models
│   ├── database.go         # Database struct, Load/Save, mutex, CRUD helpers
│   ├── models.go           # Profile, Document, ShareRecord structs
│   └── query.go            # Query parsing and execution logic
├── config/                 # Configuration loading logic (optional, could be in main.go)
│   └── config.go
├── utils/                  # Helpers
│   ├── auth.go             # JWT generation/validation, password hashing
│   ├── logging.go          # Simple colorful logging setup (optional)
│   └── utils.go            # UUIDs (dashless), error handling helpers, etc.
└── database.json           # Default data file (created on first save if default path used)
└── docs.key                # Default generated JWT key file (created if needed)
└── plan.md                 # This file
```
*(Note: `database.json` and `docs.key` should be added to `.gitignore`)*

## 8. Logging

*   **Goal:** Simple, human-readable output suitable for academic demonstration. Performance is not a primary concern.
*   **Format:** Log messages should clearly indicate the event (e.g., server start, request received, DB save, error occurred). Include relevant context (e.g., request path, error details).
*   **Color:** Use ANSI color codes to differentiate log levels (e.g., INFO, WARN, ERROR) or specific events (e.g., OTP generation) for better readability in the console.
*   **Implementation:** Use Go's standard `log` package or a simple third-party library that supports basic coloring (e.g., `fatih/color`). Avoid complex structured logging frameworks.
*   **Key Events to Log:**
    *   Server startup (including address, port, configuration loaded).
    *   Database loading/saving events (start, success, failure, retry).
    *   JWT key generation/loading.
    *   OTP generation (including the OTP value, clearly marked).
    *   Significant errors (e.g., unhandled panics, critical failures).
    *   Optionally: Incoming requests (method, path).

## 9. General Implementation Notes

*   **UUIDs:** Use `google/uuid`. Generate new UUIDs using `uuid.NewString()`. **Crucially, remove dashes** from the string representation before storing or using in API paths/responses (e.g., `strings.ReplaceAll(uuid.NewString(), "-", "")`). Ensure all references to IDs (in models, API paths, request/response bodies, JWT claims) use this dashless format consistently.
*   **Timestamps:** Use `time.Now().UTC()` for all creation and modification timestamps to ensure consistency. Store as `time.Time` in structs, which will be marshalled to RFC3339 format in JSON by default.
*   **Input Validation:** Use Gin's built-in validation (`binding:"required"`, etc.) for request bodies and path/query parameters where appropriate. Return 400 Bad Request for validation errors.
*   **Error Handling:** Use helper functions or middleware to standardize error responses (e.g., returning JSON like `{"error": "message"}` with appropriate HTTP status codes).
*   **CORS:** Not required for this project.

## 10. High-Level Flow (Mermaid)

```mermaid
graph TD
    A[Client] -- HTTP Request --> B(Gin Router);

    subgraph Public Routes
        B -- /auth/... --> C[Auth Handlers];
    end

    subgraph Protected Routes
        B -- /profiles | /documents | /shares --> D[Auth Middleware];
        D -- Valid JWT --> E[Resource Handlers (Profile/Doc/Share)];
        D -- Invalid JWT --> F[401 Unauthorized];
    end

    subgraph Backend
        C -- Uses --> G[DB Module];
        E -- Uses --> G[DB Module];
        G -- Manages --> H[In-Memory Data (Maps)];
        G -- Reads/Writes --> I[database.json];
        G -- Uses --> J[Query Engine];
        G -- Uses --> K[sync.RWMutex];
        G -- Uses --> L[Auth Utils (Hashing/JWT)];
        C -- Uses --> L;
        D -- Uses --> L;
    end

    style Public Routes fill:#f9f,stroke:#333,stroke-width:2px
    style Protected Routes fill:#ccf,stroke:#333,stroke-width:2px
    style Backend fill:#cfc,stroke:#333,stroke-width:2px
```

## 11. Testing Strategy

Thorough testing is crucial for ensuring the API's correctness, reliability, and security. The following testing approaches will be employed:

*   **Unit Tests:**
    *   **Location:** Alongside the code they test (e.g., `db/database_test.go`, `utils/auth_test.go`).
    *   **Scope:** Focus on individual functions and components in isolation.
        *   Database CRUD operations (mocking file I/O where necessary).
        *   Query parsing and evaluation logic.
        *   Authentication helpers (JWT generation/validation, password hashing/comparison, OTP generation/validation).
        *   Utility functions (including dashless UUID handling).
        *   Configuration loading logic.
        *   Logging helpers (if any).
    *   **Tools:** Go's standard `testing` package.

*   **Integration Tests:**
    *   **Location:** Potentially in a separate `tests/` directory or within `api/` (e.g., `api/handlers_test.go`).
    *   **Scope:** Test the interaction between different components, primarily focusing on the API endpoints.
        *   Set up a test instance of the Gin router and the database (using temporary test database files and potentially test-specific configuration).
        *   Use Go's `net/http/httptest` package to make HTTP requests to the test server.
        *   Verify HTTP status codes, response bodies, and database state changes. Ensure IDs in responses/paths are dashless.
    *   **Coverage:**
        *   **Happy Paths:** Test successful execution of each endpoint with valid inputs and authentication.
        *   **Error Handling:** Test responses for invalid inputs (malformed JSON, incorrect types, validation errors), missing parameters, etc. (expecting 400/422).
        *   **Authentication/Authorization:**
            *   Test accessing protected endpoints without a token (expect 401).
            *   Test accessing protected endpoints with an invalid/expired token (expect 401).
            *   Test authorization rules (e.g., user A cannot access/modify user B's documents unless shared, owner-only actions like sharing).
        *   **Edge Cases:** Test boundaries for pagination (`limit`), empty results, etc.
        *   **Database Interaction:** Verify that API calls correctly create, read, update, and delete data in the test database.
        *   **Configuration:** Test server startup with different configuration options (CLI args, ENV vars, defaults).