# DocServer

## Purpose

DocServer is a simple API server designed for **educational purposes only**. It demonstrates basic concepts of:

*   User authentication (Signup, Login, JWT-based sessions, Password Reset)
*   Document storage (storing arbitrary JSON content)
*   Document sharing between users
*   Content-based querying of stored JSON documents

**⚠️ WARNING: This server is NOT intended for production use.** It uses a simple JSON file for data persistence and lacks many security features and optimizations required for a production environment.

## Features

*   **User Management:** Register, login, update profile, delete profile.
*   **Document CRUD:** Create, Read, Update, Delete JSON documents.
*   **Document Sharing:** Share documents with other users, manage share permissions.
*   **Content Querying:** Filter documents based on the structure and values within their JSON content using a flexible query language (see API documentation for details).
*   **JWT Authentication:** Uses JSON Web Tokens for session management.
*   **File-based Persistence:** Stores all data (profiles, documents, shares) in a single JSON file (`docs.json` by default).
*   **Configurable:** Settings can be adjusted via command-line arguments or environment variables.

## Getting Started

### Prerequisites

*   Go (version 1.18 or later recommended)

### Running the Server

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd docserver
    ```
2.  **Run directly using `go run`:**
    ```bash
    go run main.go [arguments...]
    ```
    The server will start, typically listening on `0.0.0.0:8080` by default.

### Building the Server

1.  **Build the binary:**
    ```bash
    go build -o docserver main.go
    ```
    This will create an executable file named `docserver` (or `docserver.exe` on Windows).
2.  **Run the compiled binary:**
    ```bash
    ./docserver [arguments...]
    ```

### Downloading Pre-compiled Binaries

Pre-compiled binaries for different operating systems may be available on the [GitHub Releases page](<your-github-repo-url>/releases) (Replace `<your-github-repo-url>` with the actual repository URL).

## Configuration

The server can be configured using command-line arguments or environment variables. Command-line arguments take precedence over environment variables, which take precedence over default values.

| Argument          | Environment Variable | Default         | Description                                                                 |
| :---------------- | :------------------- | :-------------- | :-------------------------------------------------------------------------- |
| `-address`        | `ADDRESS`            | `0.0.0.0`       | Server listen address                                                       |
| `-port`           | `PORT`               | `8080`          | Server listen port                                                          |
| `-db-file`        | `DB_FILE`            | `./docs.json`   | Path to the JSON database file                                              |
| `-save-interval`  | `SAVE_INTERVAL`      | `3s`            | Debounce interval for saving the database (e.g., `5s`, `100ms`)             |
| `-enable-backup`  | `ENABLE_BACKUP`      | `true`          | Enable database backup (`.bak` file) before saving (`true` or `false`)      |
| `-jwt-secret-file`| `JWT_SECRET_FILE`    | *(none)*        | Path to a file containing the JWT secret key                                |
| *(none)*          | `JWT_SECRET`         | *(none)*        | The JWT secret key as an environment variable                               |

**JWT Secret Handling:**

The JWT secret used to sign authentication tokens is determined in the following order of priority:

1.  **`-jwt-secret-file` Argument / `JWT_SECRET_FILE` Env Var:** If specified, the secret is read from this file.
2.  **`JWT_SECRET` Env Var:** If the file is not specified or fails to load, the secret is read from this environment variable.
3.  **Generated Secret:** If neither a file nor an environment variable provides a secret, a new random secret is generated.
    *   The server will attempt to save this generated secret to `./docs.key`.
    *   **Important:** If a secret is generated, ensure the `./docs.key` file persists across server restarts, or users will be logged out. Add this file to your `.gitignore`.

## API Documentation

Once the server is running, interactive API documentation (Swagger UI) is available at:

`http://<server-address>:<server-port>/swagger/index.html`

(e.g., `http://localhost:8080/swagger/index.html`)

This documentation provides details on all available endpoints, request/response formats, and includes the specifics of the `content_query` syntax.