# DocServer

## Purpose

DocServer is a simple API server designed for **educational purposes only**. It demonstrates basic concepts of:

* User authentication (Signup, Login, JWT-based sessions, Password Reset)
* Document storage (storing arbitrary JSON content)
* Document sharing between users
* Content-based querying of stored JSON documents

**⚠️ WARNING: This server is NOT intended for production use.** It uses a simple JSON file for data persistence and lacks many security features and optimizations required for a production environment.

## Features

* **User Management:** Register, login, update profile, delete profile.
* **Document CRUD:** Create, Read, Update, Delete JSON documents.
* **Document Sharing:** Share documents with other users, manage share permissions.
* **Content Querying:** Filter documents based on the structure and values within their JSON content using a flexible query language (see API documentation for details).
* **JWT Authentication:** Uses JSON Web Tokens for session management.
* **File-based Persistence:** Stores all data (profiles, documents, shares) in a single JSON file (`docs.json` by default).
* **Configurable:** Settings can be adjusted via command-line arguments or environment variables.

## Getting Started

Here's a quick overview:

1. [Download the appropriate binary](#downloading-pre-compiled-binaries) for your system.
2. [Run the downloaded binary](#running-the-binary) from your terminal.

See below for detailed instructions.

### 1) Downloading Pre-compiled Binaries

Pre-compiled binaries for different operating systems may be available on the [GitHub Releases page](https://github.com/HWilliams64/docserver/releases).

#### Choosing the Right Binary

The binaries are provided for different combinations of Operating System (OS) and CPU Architecture. Here's a guide to help you choose:

| OS      | amd64 (x86_64)                                  | arm64 (aarch64)                                      |
| :------ | :---------------------------------------------- | :--------------------------------------------------- |
| Windows | Most desktop and laptop PCs (Intel/AMD processors). If unsure, choose this. | Newer PCs or tablets specifically using ARM chips (e.g., Surface Pro X, some newer Dell/Lenovo models). |
| Mac     | Macs with an Intel processor (Generally models from Mid 2020 or earlier). | Macs with Apple Silicon (M1, M2, M3, etc. chips, generally models from Late 2020 onwards). |
| Linux   | Most desktop PCs, laptops, and servers (Intel/AMD processors). | Devices like Raspberry Pi (versions 3 and newer), many modern Android phones/tablets (if running Linux), some newer servers, or other ARM-based single-board computers. |

* __amd64 (also known as x86_64):__ This is the most common architecture for desktop and laptop computers running Windows or Linux, and for older Mac computers.
* **arm64 (also known as aarch64):** This architecture is common in mobile devices (like phones and tablets), newer Mac computers (Apple Silicon), Raspberry Pi devices, and some newer Windows laptops/tablets.

#### Release File Naming Convention

The release files follow the pattern `docserver-<OS>-<ARCH>[.extension]`:

* `<OS>`: The target operating system (`linux`, `windows`, or `darwin` for Mac).
* `<ARCH>`: The CPU architecture (`amd64` or `arm64`).
* `[.extension]`: Windows executables have a `.exe` extension. Linux and Mac executables typically have no extension.

For example:

* `docserver-windows-amd64.exe`: For standard 64-bit Windows.
* `docserver-darwin-arm64`: For Macs with Apple Silicon (M1/M2/M3).
* `docserver-linux-amd64`: For standard 64-bit Linux.

### 2) Running the Binary

After downloading the appropriate binary:

1. **Open your terminal or command prompt.**

2. **Navigate to the directory where you downloaded the file.**

3. **Run the binary:**

* **Windows (Command Prompt or PowerShell):**

```bash
.\docserver-windows-<ARCH>.exe [arguments...]
```

*(e.g., `.\docserver-windows-amd64.exe -port 9000`)*

* **Mac/Linux (Terminal):**

```bash
./docserver-<OS>-<ARCH> [arguments...]
```

*(e.g., `./docserver-darwin-arm64 -port 9000` or `./docserver-linux-amd64 --db-file /data/mydocs.json`)*

Refer to the [Configuration](#configuration) section below for available arguments like `-port` or `-db-file`.

## Classroom Resources

If you are using DocServer in a classroom setting or for structured learning, additional resources are available in the `docs/class` directory:

* **`docs/class/what-is-rest.md`**: An introduction to the concepts of REST APIs.
* **`docs/class/slides.md`**: Presentation slides covering the DocServer API and its usage.
* **`docs/class/workbook.md`**: An interactive workbook with step-by-step exercises to practice using the DocServer API via `curl`.

These resources provide context and hands-on practice to complement the core API functionality.

## Configuration

The server can be configured using command-line arguments or environment variables. Command-line arguments take precedence over environment variables, which take precedence over default values.

| Argument          | Environment Variable | Default         | Description                                                                 |
| :---------------- | :------------------- | :-------------- | :-------------------------------------------------------------------------- |
| `-address`        | `ADDRESS`            | `0.0.0.0`       | Server listen address                                                       |
| `-port`           | `PORT`               | `8080`          | Server listen port                                                          |
| `-db-file`        | `DB_FILE`            | `./docs.json`   | Path to the JSON database file                                              |
| `-save-interval`  | `SAVE_INTERVAL`      | `3s`            | Debounce interval for saving the database (e.g., `5s`, `100ms`)             |
| `-enable-backup`  | `ENABLE_BACKUP`      | `true`          | Enable database backup (`.bak` file) before saving (`true` or `false`)      |
| `-jwt-secret-file`| `JWT_SECRET_FILE`    | _(none)_        | Path to a file containing the JWT secret key                                |
| _(none)_          | `JWT_SECRET`         | _(none)_        | The JWT secret key as an environment variable                               |

**JWT Secret Handling:**

The JWT secret used to sign authentication tokens is determined in the following order of priority:

1. __`-jwt-secret-file` Argument / `JWT_SECRET_FILE` Env Var:__ If specified, the secret is read from this file.
2. __`JWT_SECRET` Env Var:__ If the file is not specified or fails to load, the secret is read from this environment variable.
3. **Generated Secret:** If neither a file nor an environment variable provides a secret, a new random secret is generated.
   * The server will attempt to save this generated secret to `./docs.key`.
   * **Important:** If a secret is generated, ensure the `./docs.key` file persists across server restarts, or users will be logged out. Add this file to your `.gitignore`.

## Authentication

Authentication for protected API endpoints is handled using JSON Web Tokens (JWT).

1. **Login:** Obtain a JWT by sending valid user credentials to the `/login` endpoint.

2. **Authorization Header:** For subsequent requests to protected endpoints, include the obtained JWT in the `Authorization` header using the `Bearer` scheme:

```sh
Authorization: Bearer <your_jwt_token>
```

### Authentication Flow

Here's a typical sequence for accessing protected resources like documents:

```mermaid
sequenceDiagram
    participant Client
    participant Server

    %% Signup %%
    Client->>Server: POST /auth/signup (email, password, name)
    Server-->>Server: Hash password, Create profile
    Server->>Client: 201 Created (Profile details)

    %% Login %%
    Client->>Server: POST /auth/login (email, password)
    Server-->>Server: Verify credentials, Generate JWT
    Server->>Client: 200 OK (JWT Token)

    %% Access Protected Resource (e.g., Create Document) %%
    Client->>Server: POST /documents (Authorization: Bearer <JWT Token>, content)
    Server-->>Server: Middleware: Validate JWT
    alt JWT Valid
        Server-->>Server: Process request (e.g., Create document)
        Server->>Client: 201 Created (Document details)
    else JWT Invalid/Missing
        Server->>Client: 401 Unauthorized
    end
```

**Explanation:**

1. **Signup:** The user first creates an account using their email, password, and name via the `/auth/signup` endpoint. The server hashes the password and stores the user profile.
2. **Login:** The user then logs in using their email and password via `/auth/login`. If the credentials are correct, the server generates a JWT (JSON Web Token) and sends it back to the client.
3. **Store Token:** The client application (e.g., browser, mobile app) must securely store this JWT.
4. **Authenticated Request:** To access a protected endpoint (like `POST /documents` to create a document, or `GET /documents/{id}` to retrieve one), the client must include the stored JWT in the `Authorization` header of the request, using the `Bearer` scheme (e.g., `Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...`).
5. **Server Verification:** The server's authentication middleware intercepts the request, extracts the JWT from the header, and verifies its signature and expiration.
6. **Access Granted/Denied:** If the JWT is valid, the middleware allows the request to proceed to the intended handler (e.g., `CreateDocumentHandler`). If the JWT is missing, invalid, or expired, the server rejects the request with a `401 Unauthorized` error.

## API Documentation

Once the server is running, interactive API documentation (Swagger UI) is available at:

`http://<server-address>:<server-port>/docs/index.html`

(e.g., `http://localhost:8080/docs/index.html`)

This documentation provides details on all available endpoints, request/response
formats, and includes the specifics of the `content_query` syntax.

## Running Source

**Note:** For most users, downloading and running a [pre-compiled binary](#downloading-pre-compiled-binaries) is the easiest way to get started. The instructions below are primarily for developers who want to modify the code or build the server themselves.

#### Prerequisites

* Go (version 1.22 or later recommended)

#### Running the Server

1. **Clone the repository:**

```bash
git clone https://github.com/HWilliams64/docserver/releases
cd docserver
```

2. **Run directly using `go run`:**

```bash
go run main.go [arguments...]
```

The server will start, typically listening on `0.0.0.0:8080` by default.

#### Building the Server

1. **Build the binary:**

```bash
go build -o docserver main.go
```

This will create an executable file named `docserver` (or `docserver.exe` on Windows).

2. **Run the compiled binary:**

```bash
./docserver [arguments...]
```

### Running Tests

The project includes unit and integration tests. To run all tests:

1. **Ensure you are in the project's root directory** (the `docserver` directory containing `go.mod`).

2. **Run the Go test command:**

```bash
go test ./...
```

This command discovers and runs all tests (`*_test.go` files) within the current directory and all its subdirectories.

* **Unit Tests:** These typically test individual functions or components in isolation (e.g., testing utility functions, database query logic).
* __Integration Tests:__ These test the interaction between different parts of the system, often involving setting up a test server and making actual API calls (e.g., testing the full signup-login-create document flow). The integration tests are located in the `integration_tests` directory.
