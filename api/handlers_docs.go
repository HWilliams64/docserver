package api

import (
	"docserver/config"
	"docserver/db"
	"docserver/models"
	"docserver/utils"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// --- Create Document ---

// CreateDocumentRequest defines the expected body for creating a document.
type CreateDocumentRequest struct {
	Content any `json:"content" binding:"required"` // Content can be any valid JSON
}

// CreateDocumentHandler handles the creation of a new document.
// @Summary      Create a New Document
// @Description  Allows a logged-in user to create and store a new document.
// @Description
// @Description  The document's `content` can be any valid JSON structure – an object (`{}`), an array (`[]`), a string (`""`), a number, a boolean (`true`/`false`), or `null`.
// @Description  The server automatically assigns a unique ID to the document and records the user who created it (the owner) and the creation/modification timestamps.
// @Description  You must provide your access token for authentication. The request body needs a `content` field containing the JSON data you want to store.
// @Description
// @Description  Example Request Body:
// @Description  ```json
// @Description  {
// @Description    "content": {
// @Description      "title": "My First Document",
// @Description      "body": "This is the content.",
// @Description      "tags": ["example", "getting started"]
// @Description    }
// @Description  }
// @Description  ```
// @Tags         Documents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        document body CreateDocumentRequest true "The JSON content you want to store in the new document."
// @Success      201  {object}  models.Document "Document Created Successfully. The response body contains the details of the newly created document, including its unique ID."
// @Failure      400  {object}  utils.APIError "Bad Request: The request body is invalid. It must be valid JSON and contain the required 'content' field."
// @Failure      401  {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired. You need to be logged in to create documents."
// @Failure      500  {object}  utils.APIError "Internal Server Error: Something went wrong on the server while creating the document (e.g., database error)."
// @Router       /documents [post]
func CreateDocumentHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.GinInternalServerError(c, "User ID not found in context.")
		return
	}
	userIDStr := userID.(string)

	var req CreateDocumentRequest
	// Use BindJSON here as ShouldBindJSON might consume the body needed for later gjson validation if we add it
	if err := c.BindJSON(&req); err != nil {
		// Check if content is just plain text (not valid JSON) - this might be allowed?
		// Plan says "Can be any JSON structure or simple text".
		// Let's assume binding requires valid JSON for now, but allow flexibility later if needed.
		utils.GinBadRequest(c, fmt.Sprintf("Invalid request body: %v. 'content' must be provided.", err))
		return
	}

	// Create the document model
	doc := models.Document{
		OwnerID: userIDStr,
		Content: req.Content,
		// ID and timestamps are set by db.CreateDocument
	}

	// Save to database
	createdDoc, err := database.CreateDocument(doc)
	if err != nil {
		utils.GinInternalServerError(c, fmt.Sprintf("Failed to create document: %v", err))
		return
	}

	c.JSON(http.StatusCreated, createdDoc)
}

// --- Get Documents (List with Querying) ---

// GetDocumentsResponse defines the structure for the paginated document list results.
type GetDocumentsResponse struct {
	Data  []models.Document `json:"data"`
	Total int               `json:"total"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
}

// GetDocumentsHandler handles retrieving a list of documents based on query parameters.
// @Summary      List and Search Your Documents
// @Description  Retrieves a list of documents that the currently logged-in user has access to (either owned or shared with them).
// @Description
// @Description  This endpoint supports powerful filtering, sorting, and pagination using query parameters:
// @Description  *   `scope`: Control which documents to see:
// @Description      *   `owned`: Only documents you created.
// @Description      *   `shared`: Only documents shared with you by others.
// @Description      *   `all` (default): Both owned and shared documents.
// @Description  *   `content_query`: Filter documents based on their JSON content using a specific query language (details likely in separate documentation or examples). This allows searching within the document data itself. Example: `?content_query=metadata.status eq "published"`
// @Description  *   `sort_by`: Choose the field to sort results by: `creation_date` (default) or `last_modified_date`.
// @Description  *   `order`: Set the sort direction: `asc` (ascending) or `desc` (descending, default).
// @Description  *   `page`: For pagination, specify the page number (starts at 1, default is 1).
// @Description  *   `limit`: For pagination, specify the number of documents per page (default is 20, max is 100).
// @Description
// @Description  Example: `/documents?scope=owned&sort_by=last_modified_date&order=asc&page=1&limit=10` (Get the first 10 oldest modified documents owned by the user).
// @Tags         Documents
// @Produce      json
// @Security     BearerAuth
// @Param        scope         query     string  false  "Filter by ownership: 'owned', 'shared', or 'all'." Enums(owned, shared, all) default(all) example(owned)
// @Param        content_query query     []string false "Advanced filter based on document content (specific syntax applies)." collectionFormat(multi) example(user.name eq "John Doe")
// @Param        sort_by       query     string  false  "Field to sort results by." Enums(creation_date, last_modified_date) default(creation_date) example(last_modified_date)
// @Param        order         query     string  false  "Sorting direction." Enums(asc, desc) default(desc) example(asc)
// @Param        page          query     int     false  "Page number for pagination (starts at 1)." minimum(1) default(1) example(2)
// @Param        limit         query     int     false  "Number of documents per page." minimum(1) maximum(100) default(20) example(50)
// @Success      200  {object}  GetDocumentsResponse "A list of documents matching the criteria, along with pagination details (total count, current page, limit)."
// @Failure      400  {object}  utils.APIError "Bad Request: One or more query parameters are invalid (e.g., invalid 'scope', incorrect 'content_query' syntax, non-integer 'page'/'limit')."
// @Failure      401  {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired."
// @Failure      500  {object}  utils.APIError "Internal Server Error: Something went wrong on the server while retrieving documents."
// @Router       /documents [get]
func GetDocumentsHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.GinInternalServerError(c, "User ID not found in context.")
		return
	}
	userIDStr := userID.(string)

	// Parse query parameters
	scope := c.DefaultQuery("scope", "all") // owned, shared, all
	contentQuery := c.QueryArray("content_query") // Expects ?content_query=path op val&content_query=logic&...
	sortBy := c.DefaultQuery("sort_by", "creation_date") // creation_date, last_modified_date
	order := c.DefaultQuery("order", "desc") // asc, desc
	pageQuery := c.DefaultQuery("page", "1")
	limitQuery := c.DefaultQuery("limit", "20")

	page, errPage := strconv.Atoi(pageQuery)
	limit, errLimit := strconv.Atoi(limitQuery)

	if errPage != nil || errLimit != nil || page < 1 {
		utils.GinBadRequest(c, "Invalid 'page' or 'limit' query parameter. Must be positive integers.")
		return
	}

	// Prepare params for database query
	params := db.QueryDocumentsParams{
		AuthUserID:   userIDStr,
		Scope:        scope,
		ContentQuery: contentQuery,
		SortBy:       sortBy,
		Order:        order,
		Page:         page,
		Limit:        limit, // Max limit enforced by db.QueryDocuments/paginateDocuments
	}

	// Execute query
	docs, totalMatching, err := database.QueryDocuments(params)
	if err != nil {
		// Check for specific query-related errors (e.g., bad syntax, invalid scope)
		if strings.Contains(err.Error(), "invalid content_query") ||
		   strings.Contains(err.Error(), "invalid scope value") ||
		   strings.Contains(err.Error(), "invalid sort_by value") ||
		   strings.Contains(err.Error(), "invalid order value") ||
		   strings.Contains(err.Error(), "error evaluating content query") {
			utils.GinBadRequest(c, err.Error())
		} else {
			utils.GinInternalServerError(c, fmt.Sprintf("Failed to query documents: %v", err))
		}
		return
	}

	// Return paginated list and total count using the defined struct
	c.JSON(http.StatusOK, GetDocumentsResponse{
		Data:  docs,
		Total: totalMatching,
		Page:  page,
		Limit: params.Limit, // Return the potentially capped limit
	})
}

// --- Get Document by ID ---

// GetDocumentByIDHandler handles retrieving a single document by its ID.
// @Summary      Get a Specific Document by ID
// @Description  Retrieves the full details of a single document using its unique identifier (`id`).
// @Description
// @Description  You can only retrieve a document if:
// @Description  1. You are the owner of the document.
// @Description  OR
// @Description  2. The document has been explicitly shared with you by its owner.
// @Description
// @Description  Provide the document's `id` as part of the URL path. You also need your access token for authentication.
// @Tags         Documents
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "The unique identifier of the document you want to retrieve." example(doc_abc123xyz)
// @Success      200  {object}  models.Document "Successfully retrieved the document. The response body contains the document's details (ID, owner, content, timestamps)."
// @Failure      400  {object}  utils.APIError "Bad Request: The document ID provided in the URL path is missing or invalid."
// @Failure      401  {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired."
// @Failure      403  {object}  utils.APIError "Forbidden: You do not have permission to view this document. You are neither the owner nor has it been shared with you."
// @Failure      404  {object}  utils.APIError "Not Found: No document exists with the specified ID."
// @Failure      500  {object}  utils.APIError "Internal Server Error: Something went wrong on the server while retrieving the document."
// @Router       /documents/{id} [get]
func GetDocumentByIDHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.GinInternalServerError(c, "User ID not found in context.")
		return
	}
	userIDStr := userID.(string)
	docID := c.Param("id") // Get ID from path

	if docID == "" {
		utils.GinBadRequest(c, "Document ID is required in the path.")
		return
	}

	// Retrieve document from database
	doc, found := database.GetDocumentByID(docID)
	if !found {
		utils.GinNotFound(c, fmt.Sprintf("Document with ID '%s' not found.", docID))
		return
	}

	// Authorization Check: Is user the owner OR is it shared with them?
	isOwner := doc.OwnerID == userIDStr
	isShared := false
	if !isOwner {
		shareRecord, shareFound := database.GetShareRecordByDocumentID(docID)
		if shareFound {
			for _, sharedID := range shareRecord.SharedWith {
				if sharedID == userIDStr {
					isShared = true
					break
				}
			}
		}
	}

	if !isOwner && !isShared {
		utils.GinForbidden(c, "You do not have permission to access this document.")
		return
	}

	// Return the document
	c.JSON(http.StatusOK, doc)
}

// --- Update Document ---

// UpdateDocumentRequest defines the body for updating a document.
// Only content can be updated via this endpoint.
type UpdateDocumentRequest struct {
	Content any `json:"content" binding:"required"`
}

// UpdateDocumentHandler handles updating a document's content.
// @Summary      Update a Document's Content
// @Description  Replaces the *entire* existing content of a specific document with new content.
// @Description
// @Description  **Important:** This operation overwrites the previous content completely. If you only want to modify parts of the content, you should first retrieve the document, make changes to the content in your application, and then use this endpoint to save the full, modified content.
// @Description
// @Description  Only the user who originally created (owns) the document is allowed to update it.
// @Description  Provide the document's `id` in the URL path and the new JSON `content` in the request body. Authentication via access token is required.
// @Description
// @Description  Example Request Body:
// @Description  ```json
// @Description  {
// @Description    "content": { "message": "Updated content here!" }
// @Description  }
// @Description  ```
// @Tags         Documents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                true  "The unique identifier of the document to update." example(doc_abc123xyz)
// @Param        document body      UpdateDocumentRequest true  "The new JSON content to replace the existing document content."
// @Success      200      {object}  models.Document       "Document Updated Successfully. The response body contains the complete document with the updated content and modification timestamp."
// @Failure      400      {object}  utils.APIError   "Bad Request: The document ID in the path is missing/invalid, or the request body is invalid (must contain 'content' field with valid JSON)."
// @Failure      401      {object}  utils.APIError   "Unauthorized: Your access token is missing, invalid, or expired."
// @Failure      403      {object}  utils.APIError   "Forbidden: You are not the owner of this document, so you cannot update it."
// @Failure      404      {object}  utils.APIError   "Not Found: No document exists with the specified ID."
// @Failure      500      {object}  utils.APIError   "Internal Server Error: Something went wrong on the server while updating the document."
// @Router       /documents/{id} [put]
func UpdateDocumentHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.GinInternalServerError(c, "User ID not found in context.")
		return
	}
	userIDStr := userID.(string)
	docID := c.Param("id")

	if docID == "" {
		utils.GinBadRequest(c, "Document ID is required in the path.")
		return
	}

	// Bind request body
	var req UpdateDocumentRequest
	if err := c.BindJSON(&req); err != nil {
		utils.GinBadRequest(c, fmt.Sprintf("Invalid request body: %v. 'content' must be provided.", err))
		return
	}

	// Authorization Check: Only owner can update
	existingDoc, found := database.GetDocumentByID(docID)
	if !found {
		utils.GinNotFound(c, fmt.Sprintf("Document with ID '%s' not found.", docID))
		return
	}
	if existingDoc.OwnerID != userIDStr {
		utils.GinForbidden(c, "You do not have permission to update this document.")
		return
	}

	// Perform update in database
	updatedDoc, err := database.UpdateDocument(docID, req.Content)
	if err != nil {
		// Should only be "not found" if deleted between check and update, but handle anyway
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.GinNotFound(c, err.Error())
		} else {
			utils.GinInternalServerError(c, fmt.Sprintf("Failed to update document: %v", err))
		}
		return
	}

	c.JSON(http.StatusOK, updatedDoc)
}

// --- Delete Document ---

// DeleteDocumentHandler handles deleting a document.
// @Summary      Delete a Document
// @Description  Permanently deletes a specific document from the system.
// @Description
// @Description  **WARNING: This action is irreversible!** Once deleted, the document cannot be recovered.
// @Description  Any records indicating this document was shared with others will also be removed.
// @Description
// @Description  Only the user who originally created (owns) the document is allowed to delete it.
// @Description  Provide the document's `id` in the URL path. Authentication via access token is required.
// @Tags         Documents
// @Security     BearerAuth
// @Param        id   path      string  true  "The unique identifier of the document to delete." example(doc_abc123xyz)
// @Success      204  "Document Deleted Successfully. No content is returned in the response body because the resource no longer exists."
// @Failure      400  {object}  utils.APIError "Bad Request: The document ID provided in the URL path is missing or invalid."
// @Failure      401  {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired."
// @Failure      403  {object}  utils.APIError "Forbidden: You are not the owner of this document, so you cannot delete it."
// @Failure      404  {object}  utils.APIError "Not Found: No document exists with the specified ID. (Note: The API might return 204 even if not found, treating deletion of a non-existent item as success)."
// @Failure      500  {object}  utils.APIError "Internal Server Error: Something went wrong on the server while deleting the document."
// @Router       /documents/{id} [delete]
func DeleteDocumentHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.GinInternalServerError(c, "User ID not found in context.")
		return
	}
	userIDStr := userID.(string)
	docID := c.Param("id")

	if docID == "" {
		utils.GinBadRequest(c, "Document ID is required in the path.")
		return
	}

	// Authorization Check: Only owner can delete
	existingDoc, found := database.GetDocumentByID(docID)
	if !found {
		// Return 204 even if not found, as the end state (not existing) is achieved.
		// Or return 404? Plan suggests 204 for successful delete. Let's stick to that.
		c.Status(http.StatusNoContent)
		return
	}
	if existingDoc.OwnerID != userIDStr {
		utils.GinForbidden(c, "You do not have permission to delete this document.")
		return
	}

	// Perform delete in database (handles associated share record deletion)
	err := database.DeleteDocument(docID)
	if err != nil {
		// Should only be "not found" if deleted between check and delete.
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			// Already handled above by returning 204 if initially not found.
			// If it's not found *here*, something odd happened, but 204 is still okay.
		} else {
			utils.GinInternalServerError(c, fmt.Sprintf("Failed to delete document: %v", err))
			return // Return 500 if delete fails unexpectedly
		}
	}

	c.Status(http.StatusNoContent) // 204 No Content on successful deletion
}