package api

import (
	"docserver/config"
	"docserver/db"
	// "docserver/models" // Removed unused import
	"docserver/utils"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Helper function to check document ownership for share operations
func checkDocumentOwner(c *gin.Context, database *db.Database, docID string) (string, bool) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.GinInternalServerError(c, "User ID not found in context.")
		return "", false
	}
	userIDStr := userID.(string)

	doc, found := database.GetDocumentByID(docID)
	if !found {
		utils.GinNotFound(c, fmt.Sprintf("Document with ID '%s' not found.", docID))
		return "", false
	}

	if doc.OwnerID != userIDStr {
		utils.GinForbidden(c, "Only the document owner can manage shares.")
		return "", false
	}

	return userIDStr, true // Return owner ID and success
}

// --- Get Sharers ---

// GetSharersResponse defines the structure for the response.
type GetSharersResponse struct {
	SharedWith []string `json:"shared_with"` // List of Profile IDs (dashless)
}

// GetSharersHandler retrieves the list of profile IDs a document is shared with.
// @Summary      See Who a Document is Shared With
// @Description  Retrieves a list of user profile IDs that a specific document has been shared with.
// @Description
// @Description  Only the user who originally created (owns) the document can use this endpoint to see who they've shared it with.
// @Description  Provide the document's `id` in the URL path. Authentication via access token is required.
// @Description  If the document hasn't been shared with anyone, it returns an empty list.
// @Tags         Sharing
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "The unique identifier of the document whose share list you want to view." example(doc_abc123xyz)
// @Success      200  {object}  GetSharersResponse "Successfully retrieved the list of profile IDs the document is shared with. The 'shared_with' array contains the IDs."
// @Failure      400  {object}  utils.APIError "Bad Request: The document ID provided in the URL path is missing or invalid."
// @Failure      401  {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired."
// @Failure      403  {object}  utils.APIError "Forbidden: You are not the owner of this document, so you cannot view its share list."
// @Failure      404  {object}  utils.APIError "Not Found: No document exists with the specified ID."
// @Failure      500  {object}  utils.APIError "Internal Server Error: Something went wrong on the server while retrieving the share list."
// @Router       /documents/{id}/shares [get]
func GetSharersHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	docID := c.Param("id")
	if docID == "" {
		utils.GinBadRequest(c, "Document ID is required in the path.")
		return
	}

	// Check ownership
	if _, ok := checkDocumentOwner(c, database, docID); !ok {
		return // Error response already sent by helper
	}

	// Get the share record
	shareRecord, found := database.GetShareRecordByDocumentID(docID)
	if !found {
		// No shares exist, return empty list
		c.JSON(http.StatusOK, GetSharersResponse{SharedWith: []string{}})
		return
	}

	c.JSON(http.StatusOK, GetSharersResponse{SharedWith: shareRecord.SharedWith})
}

// --- Set/Update Sharers ---

// SetSharersRequest defines the expected body for replacing the share list.
type SetSharersRequest struct {
	// Use pointer to distinguish between empty list and not provided?
	// No, binding:"required" means it must be present, even if empty array `[]`.
	SharedWith []string `json:"shared_with" binding:"required"` // List of Profile IDs (dashless)
}

// SetSharersHandler replaces the entire list of profiles a document is shared with.
// @Summary      Set/Replace Who a Document is Shared With
// @Description  Completely replaces the list of users a specific document is shared with.
// @Description
// @Description  Provide a JSON array named `shared_with` in the request body, containing the profile IDs of the users you want to share the document with.
// @Description  **Important:** Any users previously shared with, but *not* included in the new list, will lose access.
// @Description  To remove *all* shares for a document, send an empty array: `{"shared_with": []}`.
// @Description
// @Description  Only the document owner can perform this operation. You cannot share a document with yourself (the owner).
// @Description  Provide the document's `id` in the URL path. Authentication via access token is required.
// @Description
// @Description  Example Request Body (Share with user 'user_123' and 'user_456'):
// @Description  ```json
// @Description  {
// @Description    "shared_with": ["user_123", "user_456"]
// @Description  }
// @Description  ```
// @Tags         Sharing
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id           path      string            true  "The unique identifier of the document whose share list you want to set/replace." example(doc_abc123xyz)
// @Param        shareRequest body      SetSharersRequest true  "A JSON object containing the 'shared_with' key, whose value is an array of profile IDs."
// @Success      204          "Share List Updated Successfully. No content is returned in the response body."
// @Failure      400          {object}  utils.APIError "Bad Request: The request body is invalid (e.g., missing 'shared_with' array, invalid JSON) OR you tried to include the owner's ID in the 'shared_with' list."
// @Failure      401          {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired."
// @Failure      403          {object}  utils.APIError "Forbidden: You are not the owner of this document, so you cannot modify its share list."
// @Failure      404          {object}  utils.APIError "Not Found: No document exists with the specified ID."
// @Failure      500          {object}  utils.APIError "Internal Server Error: Something went wrong on the server while updating the share list."
// @Router       /documents/{id}/shares [put]
func SetSharersHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	docID := c.Param("id")
	// If docID is empty, the route itself wasn't matched correctly by Gin
	// (e.g., /documents//shares), so NotFound is more appropriate than BadRequest.
	if docID == "" {
		utils.GinNotFound(c, "Document ID is required in the path.")
		return
	}

	// Check ownership
	ownerID, ok := checkDocumentOwner(c, database, docID)
	if !ok {
		return // Error response already sent by helper
	}

	// Bind request body
	var req SetSharersRequest
	// Use BindJSON, ShouldBindJSON might consume body if we add validation later
	if err := c.BindJSON(&req); err != nil {
		utils.GinBadRequest(c, fmt.Sprintf("Invalid request body: %v. 'shared_with' array is required.", err))
		return
	}

	// Optional: Validate that profile IDs in req.SharedWith exist and are not the owner?
	validSharers := make([]string, 0, len(req.SharedWith))
	for _, profileID := range req.SharedWith {
		profileID = strings.TrimSpace(profileID)
		if profileID == "" {
			continue // Skip empty IDs
		}
		if profileID == ownerID {
			utils.GinBadRequest(c, "Cannot share document with the owner.")
			return
		}
		// Check if profile exists (optional, can be slow)
		// _, profileFound := database.GetProfileByID(profileID)
		// if !profileFound {
		// 	 utils.GinBadRequest(c, fmt.Sprintf("Profile with ID '%s' not found.", profileID))
		// 	 return
		// }
		validSharers = append(validSharers, profileID)
	}


	// Update the share record in the database
	err := database.SetShareRecord(docID, validSharers) // Pass validated list
	if err != nil {
		// SetShareRecord currently doesn't return errors unless DB save fails unexpectedly
		utils.GinInternalServerError(c, fmt.Sprintf("Failed to update shares: %v", err))
		return
	}

	c.Status(http.StatusNoContent) // 204 No Content on success
}

// --- Add Sharer ---

// AddSharerHandler adds a single profile ID to the document's share list.
// @Summary      Share a Document with One User
// @Description  Adds a single specified user (by their `profile_id`) to the list of users who can access a specific document.
// @Description
// @Description  This operation is *additive* – it doesn't affect other users the document might already be shared with.
// @Description  It's also *idempotent*, meaning if you try to add a user who already has access, the operation succeeds without making any changes.
// @Description
// @Description  Only the document owner can perform this operation. You cannot share a document with yourself (the owner).
// @Description  Provide the document's `id` and the target user's `profile_id` in the URL path. Authentication via access token is required.
// @Tags         Sharing
// @Security     BearerAuth
// @Param        id         path      string  true  "The unique identifier of the document you want to share." example(doc_abc123xyz)
// @Param        profile_id path      string  true  "The unique identifier of the user profile you want to grant access to." example(user_123)
// @Success      204        "User Added to Share List Successfully (or was already shared with). No content is returned."
// @Failure      400        {object}  utils.APIError "Bad Request: You tried to share the document with its owner (yourself)."
// @Failure      401        {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired."
// @Failure      403        {object}  utils.APIError "Forbidden: You are not the owner of this document, so you cannot share it."
// @Failure      404        {object}  utils.APIError "Not Found: The specified Document ID or Profile ID does not exist, or the IDs were missing from the URL path."
// @Failure      500        {object}  utils.APIError "Internal Server Error: Something went wrong on the server while adding the user to the share list."
// @Router       /documents/{id}/shares/{profile_id} [put]
func AddSharerHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	docID := c.Param("id")
	profileID := c.Param("profile_id") // ID of the user to share with

	// If either ID is empty, the route is effectively not found.
	if docID == "" || profileID == "" {
		utils.GinNotFound(c, "Document ID and Profile ID are required in the path.")
		return
	}

	// Check ownership
	ownerID, ok := checkDocumentOwner(c, database, docID)
	if !ok {
		return // Error response already sent by helper
	}

	// Prevent sharing with self
	if profileID == ownerID {
		utils.GinBadRequest(c, "Cannot share document with the owner.")
		return
	}

	// Optional: Check if profileID exists?
	// _, profileFound := database.GetProfileByID(profileID)
	// if !profileFound {
	// 	 utils.GinNotFound(c, fmt.Sprintf("Profile with ID '%s' not found.", profileID))
	// 	 return
	// }

	// Add sharer in the database
	err := database.AddSharerToDocument(docID, profileID)
	if err != nil {
		utils.GinInternalServerError(c, fmt.Sprintf("Failed to add sharer: %v", err))
		return
	}

	c.Status(http.StatusNoContent) // 204 No Content on success
}

// --- Remove Sharer ---

// RemoveSharerHandler removes a single profile ID from the document's share list.
// @Summary      Stop Sharing a Document with One User
// @Description  Removes a single specified user (by their `profile_id`) from the list of users who can access a specific document.
// @Description
// @Description  This operation only affects the specified user; other users the document is shared with remain unaffected.
// @Description  It's *idempotent*, meaning if you try to remove a user who doesn't currently have access (or never did), the operation succeeds without error.
// @Description
// @Description  Only the document owner can perform this operation.
// @Description  Provide the document's `id` and the target user's `profile_id` (the one to remove) in the URL path. Authentication via access token is required.
// @Tags         Sharing
// @Security     BearerAuth
// @Param        id         path      string  true  "The unique identifier of the document you want to modify shares for." example(doc_abc123xyz)
// @Param        profile_id path      string  true  "The unique identifier of the user profile whose access you want to revoke." example(user_123)
// @Success      204        "User Removed from Share List Successfully (or was not shared with). No content is returned."
// @Failure      401        {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired."
// @Failure      403        {object}  utils.APIError "Forbidden: You are not the owner of this document, so you cannot modify its shares."
// @Failure      404        {object}  utils.APIError "Not Found: The specified Document ID or Profile ID does not exist, or the IDs were missing from the URL path."
// @Failure      500        {object}  utils.APIError "Internal Server Error: Something went wrong on the server while removing the user from the share list."
// @Router       /documents/{id}/shares/{profile_id} [delete]
func RemoveSharerHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	docID := c.Param("id")
	profileID := c.Param("profile_id") // ID of the user to remove from shares

	// If either ID is empty, the route is effectively not found.
	if docID == "" || profileID == "" {
		utils.GinNotFound(c, "Document ID and Profile ID are required in the path.")
		return
	}

	// Check ownership
	if _, ok := checkDocumentOwner(c, database, docID); !ok {
		return // Error response already sent by helper
	}

	// Remove sharer in the database
	err := database.RemoveSharerFromDocument(docID, profileID)
	if err != nil {
		utils.GinInternalServerError(c, fmt.Sprintf("Failed to remove sharer: %v", err))
		return
	}

	// Return 204 even if the profile wasn't in the list originally (idempotent)
	c.Status(http.StatusNoContent)
}