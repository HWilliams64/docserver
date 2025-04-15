package main

import (
	"docserver/api"
	"docserver/config"
	"docserver/db"
	_ "docserver/docs" // Import for side effect: registers swagger spec via init()
	"docserver/utils" // For AuthMiddleware
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware
)

// @title           DocServer API
// @version         1.0.1

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
// @description         *   `eq`: Equal to (strings, numbers, booleans, null)
// @description         *   `ne`: Not equal to
// @description         *   `gt`: Greater than (numbers)
// @description         *   `gte`: Greater than or equal to (numbers)
// @description         *   `lt`: Less than (numbers)
// @description         *   `lte`: Less than or equal to (numbers)
// @description         *   `contains`: String contains substring (case-sensitive)
// @description         *   `startswith`: String starts with prefix (case-sensitive)
// @description         *   `endswith`: String ends with suffix (case-sensitive)
// @description     *   **`value`**: The value to compare against.
// @description         *   Strings MUST be enclosed in double quotes (e.g., `\"John Doe\"`). Remember to URL-encode the query parameter string.
// @description         *   Numbers (e.g., `123`, `45.6`), booleans (`true`/`false`), and `null` should be used directly.
// @description
// @description     **Logical Operators (Combining Queries):**
// @description     You combine multiple conditions by providing multiple `content_query` parameters.
// @description     *   **`AND` (Implicit):** By default, consecutive `path operator value` queries are joined by `AND`. The document must match *all* conditions.
// @description     *   **`OR` (Explicit):** To use `OR`, add `content_query=OR` *before* the condition you want to link with OR. The `OR` applies between the condition *before* it and the condition *after* it.
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
// @description         `?content_query=status eq \"active\"`
// @description
// @description     2.  **Numeric Comparison:** Find documents where `priority` is greater than or equal to `5`.
// @description         `?content_query=priority gte 5`
// @description
// @description     3.  **Nested Field:** Find documents assigned to `Alice`.
// @description         `?content_query=assignee.name eq \"Alice\"`
// @description
// @description     4.  **Array Element:** Find documents where the first tag is `urgent`.
// @description         `?content_query=tags.0 eq \"urgent\"`
// @description
// @description     5.  **Implicit `AND`:** Find documents for project `Alpha` **AND** status `active`.
// @description         `?content_query=project eq \"Alpha\"&content_query=status eq \"active\"`
// @description
// @description     6.  **Explicit `OR`:** Find documents where status is `active` **OR** priority is less than `3`.
// @description         `?content_query=status eq \"active\"&content_query=OR&content_query=priority lt 3`
// @description
// @description     7.  **Combined `AND` and `OR`:** Find documents where (project is `Alpha` **AND** status is `active`) **OR** (priority is `10`).
// @description         `?content_query=project eq \"Alpha\"&content_query=status eq \"active\"&content_query=OR&content_query=priority eq 10`
// @description         *(Explanation: `project eq "Alpha"` AND `status eq "active"` are grouped implicitly. The `OR` then links this group to `priority eq 10`.)*
// @description
// @description     8.  **Nested Field with `AND`:** Find documents where `assignee.name` is `Alice` **AND** `metadata.reviewed` is `true`.
// @description         `?content_query=assignee.name eq \"Alice\"&content_query=metadata.reviewed eq true`

// @license.name  MIT
// @license.url   https://github.com/HWilliams64/docserver/blob/main/License.md

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.jwt BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
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
	// Serve static files (CSS, JS, swagger.json) from the docs directory
	router.StaticFS("/docs", http.Dir("docs"))
	// Use ginSwagger to handle the UI rendering, pointing it to the served swagger.json
	// Note: The path for WrapHandler needs to be different from StaticFS to avoid conflict.
	// Let's use /swagger/ for the UI itself.
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/swagger.json")))


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