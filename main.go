package main

import (
	"docserver/api"
	"docserver/config"
	"docserver/db"
	_ "docserver/docs" // Import for side effect: registers swagger spec via init()
	"docserver/utils" // For AuthMiddleware
	"embed"           // Added for embedding files
	"fmt"
	"io/fs" // Added for filesystem interface
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware
)

// @title           DocServer API
//
// @description     ## DocServer API
// @description
// @description     **Purpose:** This is a simple API server designed for **educational purposes only**. It demonstrates basic concepts of user authentication, document storage (as JSON), document sharing, and content-based querying. **It is NOT intended for production use.**
// @description
// @description     **High-Level Overview:**
// @description     DocServer allows users to:
// @description     *   Register and log in to manage their accounts.
// @description     *   Create, retrieve, update, and delete documents. Document content can be any valid JSON structure.
// @description     *   Share their documents with other registered users.
// @description     *   Search for documents they have access to, including powerful filtering based on the document's JSON content.
// @description
// @description     **Content Querying (`content_query` parameter):**
// @description     The `GET /documents` endpoint supports filtering documents based on their content using the `content_query` parameter. This allows you to search for documents where specific fields within the JSON content match certain criteria.
// @description
// @description     **Query Syntax:**
// @description     Each `content_query` parameter string follows the format: `path operator value`
// @description
// @description     *   **`path`**: A dot-separated path to navigate the JSON structure (e.g., `user.name`, `details.metadata.version`). Use numeric indices for arrays (e.g., `items.0.id`, `tags.1`).
// @description     *   **`operator`**: The comparison operator. Supported operators include:
// @description         *   `equals`: Equal to (strings, numbers, booleans, null)
// @description         *   `notequals`: Not equal to
// @description         *   `greaterthan`: Greater than (numbers)
// @description         *   `greaterthanorequals`: Greater than or equal to (numbers)
// @description         *   `lessthan`: Less than (numbers)
// @description         *   `lessthanorequals`: Less than or equal to (numbers)
// @description         *   `contains`: String contains substring, or array contains element (case-sensitive by default).
// @description         *   `startswith`: String starts with prefix (case-sensitive by default).
// @description         *   `endswith`: String ends with suffix (case-sensitive by default).
// @description     *   **`value`**: The value to compare against.
// @description         *   Strings MUST be enclosed in double quotes (e.g., `\"John Doe\"`). Remember to URL-encode the query parameter string. Add `-insensitive` suffix to string operators (e.g., `equals-insensitive`, `contains-insensitive`) for case-insensitive matching.
// @description         *   Numbers (e.g., `123`, `45.6`), booleans (`true`/`false`), and `null` should be used directly.
// @description
// @description     **Logical Operators (Combining Queries):**
// @description     You combine multiple conditions by providing `content_query` parameters for conditions interleaved with explicit logical operators (`and` or `or`).
// @description     *   **`and` (Explicit):** To link two conditions with AND, place `content_query=and` between them. The document must match *both* conditions.
// @description     *   **`or` (Explicit):** To link two conditions with OR, place `content_query=or` between them. The document must match *either* condition.
// @description
// @description     **Examples:**
// @description
// @description     *Assume document content like:*
// @description     ```json
// @description     {
// @description       "project": "Alpha",
// @description       "status": "active",
// @description       "priority": 5,
// @description       "assignee": { "name": "Alice", "email": "alice@example.com" },
// @description       "tags": ["urgent", "backend"],
// @description       "metadata": { "version": 1.2, "reviewed": true }
// @description     }
// @description     ```
// @description
// @description     1.  **Simple Equality:** Find documents where `status` is `active`.
// @description         `?content_query=status equals \"active\"`
// @description
// @description     2.  **Numeric Comparison:** Find documents where `priority` is greater than or equal to `5`.
// @description         `?content_query=priority greaterthanorequals 5`
// @description
// @description     3.  **Nested Field:** Find documents assigned to `Alice`.
// @description         `?content_query=assignee.name equals \"Alice\"`
// @description
// @description     4.  **Array Element:** Find documents where the first tag is `urgent`.
// @description         `?content_query=tags.0 equals \"urgent\"`
// @description
// @description     5.  **Explicit `AND`:** Find documents for project `Alpha` **AND** status `active`.
// @description         `?content_query=project equals \"Alpha\"&content_query=and&content_query=status equals \"active\"`
// @description
// @description     6.  **Explicit `OR`:** Find documents where status is `active` **OR** priority is less than `3`.
// @description         `?content_query=status equals \"active\"&content_query=or&content_query=priority lessthan 3`
// @description
// @description     7.  **Combined `AND` and `OR`:** Find documents where (project is `Alpha` **AND** status is `active`) **OR** (priority is `10`). Evaluation is strictly left-to-right.
// @description         `?content_query=project equals \"Alpha\"&content_query=and&content_query=status equals \"active\"&content_query=or&content_query=priority equals 10`
// @description         *(Explanation: `project equals "Alpha"` AND `status equals "active"` is evaluated first, then the result is OR'd with `priority equals 10`.)*
// @description
// @description     8.  **Nested Field with `AND`:** Find documents where `assignee.name` is `Alice` **AND** `metadata.reviewed` is `true`.
// @description         `?content_query=assignee.name equals \"Alice\"&content_query=and&content_query=metadata.reviewed equals true`
// @description Type "Bearer" followed by a space and JWT token.
//
// @license.name  MIT
// @license.url   https://github.com/HWilliams64/docserver/blob/main/License.md
//
// @host      localhost:8080
// @BasePath  /
//
// @securityDefinitions.jwt BearerAuth
// @in header
// @name Authorization


// Embed the docs directory and all its contents
//go:embed all:docs
var embeddedDocsFS embed.FS

func main() { // coverage-ignore
	// Seed random number generator (for OTPs)
	rand.Seed(time.Now().UnixNano())

	// --- Configuration ---
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("CRITICAL: Failed to load configuration: %v", err)
	}

	// --- Database ---
	database, err := db.NewDatabase(cfg)
	if err != nil {
		// NewDatabase logs specifics, including critical parse errors
		log.Fatalf("CRITICAL: Failed to initialize database: %v", err)
	}

	// --- Gin Router Setup ---
	// Consider gin.ReleaseMode for production, gin.DebugMode for development
	// gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Simple logging middleware (can be customized)
	router.Use(gin.Logger())
	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	router.Use(gin.Recovery())

	// --- Public Routes (No Auth Required) ---
	authGroup := router.Group("/auth")
	{
		// POST /auth/signup
		authGroup.POST("/signup", func(c *gin.Context) {
			api.SignupHandler(c, database, cfg)
		})
		// POST /auth/login
		authGroup.POST("/login", func(c *gin.Context) {
			api.LoginHandler(c, database, cfg)
		})
		// POST /auth/forgot-password
		authGroup.POST("/forgot-password", func(c *gin.Context) {
			api.ForgotPasswordHandler(c, database, cfg)
		})
		// POST /auth/reset-password
		authGroup.POST("/reset-password", func(c *gin.Context) {
			api.ResetPasswordHandler(c, database, cfg)
		})
	}

	// --- Protected Routes (Auth Required) ---
	// Apply AuthMiddleware
	authMiddleware := utils.AuthMiddleware(cfg)

	// Profile Routes
	profileGroup := router.Group("/profiles")
	profileGroup.Use(authMiddleware)
	{
		// GET /profiles/me
		profileGroup.GET("/me", func(c *gin.Context) {
			api.GetProfileMeHandler(c, database, cfg)
		})
		// PUT /profiles/me
		profileGroup.PUT("/me", func(c *gin.Context) {
			api.UpdateProfileMeHandler(c, database, cfg)
		})
		// DELETE /profiles/me
		profileGroup.DELETE("/me", func(c *gin.Context) {
			api.DeleteProfileMeHandler(c, database, cfg)
		})
		// GET /profiles (Search)
		profileGroup.GET("", func(c *gin.Context) { // Note: Empty path for group root
			api.SearchProfilesHandler(c, database, cfg)
		})
	}

	// Document Routes
	docGroup := router.Group("/documents")
	docGroup.Use(authMiddleware)
	{
		// POST /documents
		docGroup.POST("", func(c *gin.Context) {
			api.CreateDocumentHandler(c, database, cfg)
		})
		// GET /documents (List/Query)
		docGroup.GET("", func(c *gin.Context) {
			api.GetDocumentsHandler(c, database, cfg)
		})
		// GET /documents/{id}
		docGroup.GET("/:id", func(c *gin.Context) {
			api.GetDocumentByIDHandler(c, database, cfg)
		})
		// PUT /documents/{id}
		docGroup.PUT("/:id", func(c *gin.Context) {
			api.UpdateDocumentHandler(c, database, cfg)
		})
		// DELETE /documents/{id}
		docGroup.DELETE("/:id", func(c *gin.Context) {
			api.DeleteDocumentHandler(c, database, cfg)
		})

		// Sharing Sub-routes (nested under /documents/{id})
		shareGroup := docGroup.Group("/:id/shares")
		{
			// GET /documents/{id}/shares
			shareGroup.GET("", func(c *gin.Context) {
				api.GetSharersHandler(c, database, cfg)
			})
			// PUT /documents/{id}/shares
			shareGroup.PUT("", func(c *gin.Context) {
				api.SetSharersHandler(c, database, cfg)
			})
			// PUT /documents/{id}/shares/{profile_id}
			shareGroup.PUT("/:profile_id", func(c *gin.Context) {
				api.AddSharerHandler(c, database, cfg)
			})
			// DELETE /documents/{id}/shares/{profile_id}
			shareGroup.DELETE("/:profile_id", func(c *gin.Context) {
				api.RemoveSharerHandler(c, database, cfg)
			})
		}
	}
	
	// Logout route (needs auth middleware)
	// POST /auth/logout 
	// It's under /auth conceptually, but needs the middleware
	router.POST("/auth/logout", authMiddleware, func(c *gin.Context) {
		api.LogoutHandler(c, database, cfg)
	})

	// --- Swagger Route ---
	// Create a sub-filesystem rooted at the 'docs' directory within the embedded FS
	docsFS, err := fs.Sub(embeddedDocsFS, "docs")
	if err != nil {
		log.Fatalf("CRITICAL: Failed to create sub FS for embedded docs: %v", err)
	}
	// Serve static files from the embedded filesystem under the /docs URL path
	router.StaticFS("/static", http.FS(docsFS))

	// Use ginSwagger to handle the UI rendering, pointing it to the served swagger.json
	// The URL path remains the same as it's served via StaticFS above.
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/static/swagger.json")))


	// --- Start Server ---
	listenAddr := fmt.Sprintf("%s:%s", cfg.ListenAddress, cfg.ListenPort)
	log.Printf("INFO: Starting server on %s", listenAddr)

	// Use http.Server for graceful shutdown options if needed later
	server := &http.Server{
		Addr:    listenAddr,
		Handler: router,
		// ReadTimeout:  10 * time.Second, // Example timeouts
		// WriteTimeout: 10 * time.Second,
		// MaxHeaderBytes: 1 << 20, // 1 MB
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("CRITICAL: Server failed to start: %v", err)
	}
}