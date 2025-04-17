---
marp: true
theme: default
paginate: true
header: 'Docserver & REST APIs'
footer: 'High-Level Overview'
---

<!-- _class: lead -->
# Docserver & REST APIs
A High-Level Introduction

---

## What is a REST API?

**RE**presentational **S**tate **T**ransfer

*   An **architectural style** for designing networked applications.
*   Uses standard **HTTP methods** (GET, POST, PUT, DELETE).
*   Focuses on **Resources** (like users, documents) identified by **URIs** (like `/users/123`).
*   **Stateless:** Server doesn't remember client state between requests.

<!-- For some reason the 1st mermaid diagrams does not load so this is placeholder to the rest of the diagrams load properly--->
<pre class="mermaid" style="display: none;"></pre>

---

Think of it like ordering food: You (Client) give a specific order (Request) to the waiter (API), who brings it to the kitchen (Server), and returns with your food (Response).

<pre class="mermaid" style="background:#383838;">
sequenceDiagram
    participant Client
    participant Server
    Client->>Server: HTTP Request (e.g., GET /users/123)
    Server-->>Client: HTTP Response (e.g., 200 OK + User Data)
</pre>

---

## How REST APIs Work: Request & Response

Communication happens via HTTP messages.

<div class="mermaid">
graph TD
    subgraph HTTP Request
        ReqLine["Request Line (Method, URI, Version)"]
        ReqHeaders["Headers (Host, Auth, Content-Type, ...)"]
        ReqBody["Body (Optional - e.g., JSON data)"]
    end
    subgraph HTTP Response
        ResStatus["Status Line (Version, Status Code, Reason)"]
        ResHeaders["Headers (Content-Type, Cache-Control, ...)"]
        ResBody["Body (Optional - e.g., JSON data)"]
    end

    ReqLine --> ReqHeaders --> ReqBody
    ResStatus --> ResHeaders --> ResBody
</div>

---

*   **Method:** The action (GET, POST, etc.).
*   **URI:** The resource path (`/documents/doc_123`).
*   **Headers:** Metadata (authentication, content format).
*   **Body:** Data payload (often JSON).
*   **Status Code:** Outcome (200 OK, 404 Not Found, etc.).

---

## Example: Getting a Document

**Request:** Client asks for document `doc_abc`

```http
GET /documents/doc_abc HTTP/1.1
Host: localhost:8080
Authorization: Bearer <token>
Accept: application/json
```

---

## Example: Receiving a Document

**Response:** Server finds it and sends it back

```http
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 185

{
  "id": "doc_abc",
  "owner_id": "user_xyz",
  "content": { "title": "My Doc", "status": "draft" },
  "creation_date": "2024-01-10T10:00:00Z",
  "last_modified_date": "2024-01-10T10:00:00Z"
}
```

---

## Common REST Actions (HTTP Methods)

CRUD (Create, Read, Update, Delete) operations.

<table>
  <thead>
    <tr>
      <th>Diagram</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>
        <pre class="mermaid" style="background:#383838;">
          graph LR<br>
          &nbsp;&nbsp;POST --> Create<br>
          &nbsp;&nbsp;GET --> Read<br>
          &nbsp;&nbsp;PUT --> UpdateReplace<br>
          &nbsp;&nbsp;PATCH --> UpdateModify<br>
          &nbsp;&nbsp;DELETE --> Delete
        </pre>
      </td>
      <td>
        <ul>
          <li><strong>POST /documents</strong>: Create a new document.</li>
          <li><strong>GET /documents</strong>: List documents.</li>
          <li><strong>GET /documents/{id}</strong>: Read a specific document.</li>
          <li><strong>PUT /documents/{id}</strong>: Replace a document's content.</li>
          <li><strong>PATCH /documents/{id}</strong>: Modify part of a document.</li>
          <li><strong>DELETE /documents/{id}</strong>: Delete a document.</li>
        </ul>
      </td>
    </tr>
  </tbody>
</table>


---

## Introducing: DocServer

A simple REST API demonstrating core concepts.

**Purpose:** Store, retrieve, and share simple JSON documents.

**Key Features:**
*   User Authentication (Signup, Login) using JWT.
*   Document Management (Create, Read, Update, Delete).
*   Document Sharing between users.
*   Basic Content Querying within JSON documents.

---

## Authentication: How Docserver Knows It's You (JWT)

**Problem:** REST is stateless. How does the server know who is making a request after login?

**Solution:** JSON Web Tokens (JWT)

1.  Client logs in with email/password.
2.  Server verifies credentials.
3.  Server generates a signed JWT (a string containing user info) and sends it back.
4.  Client stores the JWT and includes it in the `Authorization: Bearer <token>` header for future requests.
5.  Server verifies the JWT signature on each request to authenticate the user.

--- 
<div style="display: flex; justify-content: center; align-items: center;">
  <pre class="mermaid" style="scale:2;background:#383838;">
    sequenceDiagram
        participant Client
        participant Server
        Client->>Server: POST /auth/login (Email, Password)
        Server->>Server: Verify Credentials
        alt Credentials OK
            Server->>Server: Generate JWT (contains UserID)
            Server-->>Client: 200 OK ({"token": "JWT_string"})
        else Credentials Invalid
            Server-->>Client: 401 Unauthorized
        end
        Client->>Client: Store JWT
        Client->>Server: GET /documents (Authorization: Bearer JWT_string)
        Server->>Server: Verify JWT Signature & Extract UserID
        Server-->>Client: 200 OK (Document List)
  </pre>
</div>



---

## Docserver Example: Signup & Login

**1. Signup Request:**

```http
POST /auth/signup HTTP/1.1
Content-Type: application/json

{
  "email": "alice@example.com",
  "password": "password123",
  "first_name": "Alice",
  "last_name": "Wonderland"
}
```

---

**Response (Success):** `201 Created`
```json
{
  "id": "user_abc",
  "first_name": "Alice",
  "last_name": "Wonderland",
  "email": "alice@example.com",
  "creation_date": "...",
  "last_modified_date": "..."
}
```

---

## Docserver Example: Signup & Login (cont.)

**2. Login Request:**

```http
POST /auth/login HTTP/1.1
Content-Type: application/json

{
  "email": "alice@example.com",
  "password": "password123"
}
```
---

**Response (Success):** `200 OK`
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```
*(Client now uses this token for authenticated requests)*

---

## Docserver Example: Creating & Getting

**1. Create Document Request (Requires Auth Header):**

```http
POST /documents HTTP/1.1
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": {
    "project": "Project X",
    "status": "Draft",
    "version": 1.0
  }
}
```

---

**Response (Success):** `201 Created`
```json
{
  "id": "doc_xyz",
  "owner_id": "user_abc",
  "content": { "project": "Project X", "status": "Draft", "version": 1.0 },
  "creation_date": "...",
  "last_modified_date": "..."
}
```

---

## Docserver Example: Creating & Getting (cont.)

**2. Get Document Request (Requires Auth Header):**

```http
GET /documents/doc_xyz HTTP/1.1
Authorization: Bearer <token>
```
**Response (Success):** `200 OK`
```json
{
  "id": "doc_xyz",
  "owner_id": "user_abc",
  "content": { "project": "Project X", "status": "Draft", "version": 1.0 },
  "creation_date": "...",
  "last_modified_date": "..."
}
```

---

## Docserver Example: Sharing

**Share `doc_xyz` with `user_bob` (Requires Auth Header):**

```http
PUT /documents/doc_xyz/shares/user_bob HTTP/1.1
Authorization: Bearer <token_from_alice>
```
*(No request body needed for adding a single user)*

**Response (Success):** `204 No Content`

*(Now `user_bob` can `GET /documents/doc_xyz` using their own token)*

---

## Docserver Example: Querying

**Get documents owned by user where content `status` is "Draft" (Requires Auth Header):**

```http
GET /documents?scope=owned&content_query=status%20equals%20%22Draft%22 HTTP/1.1
Authorization: Bearer <token>
```
*(`content_query=status equals "Draft"` - URL encoded)*

---

**Response (Success):** `200 OK`
```json
{
  "data": [
    {
      "id": "doc_xyz",
      "owner_id": "user_abc",
      "content": { "project": "Project X", "status": "Draft", "version": 1.0 },
      "creation_date": "...",
      "last_modified_date": "..."
    }
    // ... other matching documents
  ],
  "total": 1,
  "page": 1,
  "limit": 20
}
```

---

## Summary

*   **REST APIs** provide a standard, stateless way for web services to communicate using HTTP.
*   **JWT** offers a common mechanism for handling authentication in stateless APIs.
*   **Docserver** is a simple example demonstrating these concepts for creating, managing, and sharing JSON documents.

---

<!-- _class: lead -->
## Next Steps

*   Explore the interactive `docs/demo.md` workbook.
*   Try the API using the Swagger UI (run server and go to `/swagger/index.html`).
*   Examine the source code (`api/`, `db/`, `models/`).

**Thank you!**

<script type="module">
import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11.6.0/dist/mermaid.min.js';
mermaid.initialize({
    startOnLoad: true
});

window.addEventListener('vscode.markdown.updateContent', function() { mermaid.init() });
</script>