package api

import (
	"docserver/config"
	"docserver/db"
	"docserver/models"
	"docserver/utils"
	"fmt" // Added
	"net/http"
	"sort" // Added for sorting profiles
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// --- Get Current Profile ---

// GetProfileMeHandler retrieves the profile of the currently authenticated user.
// @Summary      Get Your Own Profile
// @Description  Retrieves the profile details (like first name, last name, email, creation date) for the user who is currently logged in.
// @Description
// @Description  Think of this as your "My Account" page data. To use this endpoint, you must first authenticate (log in) to get an access token.
// @Description  The server uses the access token you provide in the request header to figure out who you are and fetch your specific profile information from the database.
// @Tags         Profiles
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  models.Profile  "Your profile details were successfully retrieved. The response body contains your profile information (excluding sensitive data like the password hash)."
// @Failure      401  {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired. You might need to log in again."
// @Failure      404  {object}  utils.APIError "Not Found: The server couldn't find a profile associated with your access token. This is unusual if your token is valid."
// @Failure      500  {object}  utils.APIError "Internal Server Error: Something went wrong on the server side (e.g., a database connection issue or a problem reading your user ID from the token context)."
// @Router       /profiles/me [get]
func GetProfileMeHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	// Get userID from context (set by AuthMiddleware)
	userID, exists := c.Get("userID")
	if !exists {
		utils.GinInternalServerError(c, "User ID not found in context. Middleware issue?")
		return
	}
	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		utils.GinInternalServerError(c, "Invalid User ID format in context.")
		return
	}

	// Retrieve profile from database
	profile, found := database.GetProfileByID(userIDStr)
	if !found {
		// This shouldn't happen if the JWT is valid and the user wasn't deleted mid-session
		utils.GinError(c, http.StatusNotFound, "Authenticated user profile not found.")
		return
	}

	// Return profile (PasswordHash is excluded by `json:"-"`)
	c.JSON(http.StatusOK, profile)
}

// --- Update Profile ---

// UpdateProfileRequest defines the fields allowed for updating a profile.
// Note: Email and Password cannot be changed here. Password reset has its own flow.
type UpdateProfileRequest struct {
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Extra     any    `json:"extra,omitempty"`
}

// UpdateProfileMeHandler updates the profile of the currently authenticated user.
// @Summary      Update Your Own Profile
// @Description  Allows the currently logged-in user to update their own profile information.
// @Description
// @Description  You can change your `first_name`, `last_name`, and any custom `extra` data associated with your profile.
// @Description  **Important:** You *cannot* change your email address or password using this endpoint. Password changes typically have a separate, more secure process (like a password reset flow).
// @Description  You need to provide your current access token for authentication. The request body should contain the fields you want to update in JSON format.
// @Tags         Profiles
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        profile body UpdateProfileRequest true "The profile fields you want to update. 'first_name' and 'last_name' are required."
// @Success      200  {object}  models.Profile  "Your profile was successfully updated. The response body contains the complete, updated profile."
// @Failure      400  {object}  utils.APIError "Bad Request: The data you sent in the request body is invalid. This could be due to missing required fields ('first_name', 'last_name') or incorrect JSON formatting."
// @Failure      401  {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired. You need to be logged in to update your profile."
// @Failure      404  {object}  utils.APIError "Not Found: The server couldn't find your profile based on your access token."
// @Failure      500  {object}  utils.APIError "Internal Server Error: Something went wrong on the server while trying to update your profile (e.g., a database error)."
// @Router       /profiles/me [put]
func UpdateProfileMeHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	// Get userID from context
	userID, exists := c.Get("userID")
	if !exists {
		utils.GinInternalServerError(c, "User ID not found in context.")
		return
	}
	userIDStr := userID.(string) // Assume valid from middleware

	// Bind JSON request body
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.GinBadRequest(c, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Get the existing profile to preserve fields not being updated
	existingProfile, found := database.GetProfileByID(userIDStr)
	if !found {
		utils.GinError(c, http.StatusNotFound, "Authenticated user profile not found.")
		return
	}

	// Create the updated profile model, preserving non-updatable fields
	updatedProfileData := models.Profile{
		ID:           existingProfile.ID,
		FirstName:    req.FirstName, // Update from request
		LastName:     req.LastName,  // Update from request
		Email:        existingProfile.Email,        // Preserve original email
		PasswordHash: existingProfile.PasswordHash, // Preserve original hash
		CreationDate: existingProfile.CreationDate, // Preserve original creation date
		// LastModifiedDate will be set by db.UpdateProfile
		Extra: req.Extra, // Update from request
	}

	// Perform the update in the database
	updatedProfile, err := database.UpdateProfile(userIDStr, updatedProfileData)
	if err != nil {
		// UpdateProfile handles "not found" internally, but check just in case
		utils.GinInternalServerError(c, fmt.Sprintf("Failed to update profile: %v", err))
		return
	}

	// Return the updated profile
	c.JSON(http.StatusOK, updatedProfile)
}

// --- Delete Profile ---

// DeleteProfileMeHandler deletes the account of the currently authenticated user.
// @Summary      Delete Your Own Profile
// @Description  Permanently deletes the account and profile data for the currently logged-in user.
// @Description
// @Description  **WARNING: This action is irreversible!** Once you delete your account, all your data associated with it (profile, documents you own, etc.) will be removed.
// @Description  *(Developer Note: Full cascading delete logic, like removing shared document access, might still be under development. Currently, it primarily removes the main profile record.)*
// @Description  You must provide your valid access token to authorize this action.
// @Tags         Profiles
// @Security     BearerAuth
// @Success      204  "Account Successfully Deleted. No content is returned in the response body because the resource (your profile) no longer exists."
// @Failure      401  {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired. You need to be logged in to delete your account."
// @Failure      404  {object}  utils.APIError "Not Found: The server couldn't find your profile based on your access token."
// @Failure      500  {object}  utils.APIError "Internal Server Error: Something went wrong on the server while trying to delete your account (e.g., a database error)."
// @Router       /profiles/me [delete]
func DeleteProfileMeHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	// Get userID from context
	userID, exists := c.Get("userID")
	if !exists {
		utils.GinInternalServerError(c, "User ID not found in context.")
		return
	}
	userIDStr := userID.(string) // Assume valid from middleware

	// TODO: Implement cascading delete logic in the database layer first.
	// Deleting a profile should also delete:
	// 1. All documents owned by the user.
	// 2. All share records associated with those documents.
	// 3. Remove the user from any documents shared *with* them.

	// For now, just delete the profile record itself.
	err := database.DeleteProfile(userIDStr)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			utils.GinNotFound(c, "Authenticated user profile not found.")
		} else {
			utils.GinInternalServerError(c, fmt.Sprintf("Failed to delete profile: %v", err))
		}
		return
	}

	// Return success
	c.Status(http.StatusNoContent) // 204 No Content is appropriate for successful DELETE
}

// --- Search Profiles ---

// SearchProfilesResponse defines the structure for the paginated profile search results.
type SearchProfilesResponse struct {
	Data  []models.Profile `json:"data"`
	Total int              `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

// SearchProfilesHandler searches for profiles based on query parameters.
// @Summary      Search User Profiles
// @Description  Allows authenticated users to search for other user profiles within the system.
// @Description
// @Description  You can filter the search using query parameters in the URL:
// @Description  *   `email`: Find profiles where the email address contains the provided text (case-insensitive). Example: `?email=test.com`
// @Description  *   `first_name`: Find profiles where the first name contains the provided text (case-insensitive). Example: `?first_name=jo`
// @Description  *   `last_name`: Find profiles where the last name contains the provided text (case-insensitive). Example: `?last_name=smi`
// @Description  You can combine multiple filters. The search returns profiles that match *all* provided filters.
// @Description
// @Description  Results are paginated to handle potentially large numbers of users:
// @Description  *   `page`: Specifies which page of results to retrieve (starts at 1). Default is 1. Example: `?page=2`
// @Description  *   `limit`: Specifies how many profiles to return per page. Default is 20, maximum is 100. Example: `?limit=50`
// @Description
// @Description  Example combining filters and pagination: `/profiles?first_name=a&page=1&limit=10` (Find profiles with 'a' in the first name, show the first 10 results).
// @Tags         Profiles
// @Produce      json
// @Security     BearerAuth
// @Param        email       query     string  false  "Filter profiles where email contains this text (case-insensitive)." example(user@example.com)
// @Param        first_name  query     string  false  "Filter profiles where first name contains this text (case-insensitive)." example(John)
// @Param        last_name   query     string  false  "Filter profiles where last name contains this text (case-insensitive)." example(Doe)
// @Param        page        query     int     false  "Page number for results (starts at 1)." minimum(1) default(1) example(1)
// @Param        limit       query     int     false  "Number of profiles per page." minimum(1) maximum(100) default(20) example(20)
// @Success      200  {object}  SearchProfilesResponse "A list of profiles matching the search criteria, along with pagination details (total count, current page, limit)."
// @Failure      400  {object}  utils.APIError "Bad Request: Invalid query parameters. 'page' and 'limit' must be positive integers. 'limit' cannot exceed 100."
// @Failure      401  {object}  utils.APIError "Unauthorized: Your access token is missing, invalid, or expired. You need to be logged in to search profiles."
// @Failure      500  {object}  utils.APIError "Internal Server Error: Something went wrong on the server while searching for profiles."
// @Router       /profiles [get]
func SearchProfilesHandler(c *gin.Context, database *db.Database, cfg *config.Config) {
	// Get query parameters
	emailQuery := c.Query("email")
	firstNameQuery := c.Query("first_name")
	lastNameQuery := c.Query("last_name")
	pageQuery := c.DefaultQuery("page", "1")
	limitQuery := c.DefaultQuery("limit", "20") // Use same default as document query

	page, errPage := strconv.Atoi(pageQuery)
	limit, errLimit := strconv.Atoi(limitQuery)

	if errPage != nil || errLimit != nil || page < 1 {
		utils.GinBadRequest(c, "Invalid 'page' or 'limit' query parameter. Must be positive integers.")
		return
	}
	// Enforce max limit
	if limit > 100 {
		limit = 100
	}

	// Get all profiles (inefficient for large datasets, but simple for now)
	allProfiles := database.GetAllProfiles()

	// Filter based on query params (case-insensitive contains)
	filteredProfiles := make([]models.Profile, 0)
	for _, profile := range allProfiles {
		match := true
		if emailQuery != "" && !strings.Contains(strings.ToLower(profile.Email), strings.ToLower(emailQuery)) {
			match = false
		}
		if firstNameQuery != "" && !strings.Contains(strings.ToLower(profile.FirstName), strings.ToLower(firstNameQuery)) {
			match = false
		}
		if lastNameQuery != "" && !strings.Contains(strings.ToLower(profile.LastName), strings.ToLower(lastNameQuery)) {
			match = false
		}

		if match {
			filteredProfiles = append(filteredProfiles, profile)
		}
	}

	totalMatching := len(filteredProfiles)

	// Sort results for stable pagination (e.g., by Email then ID)
	sort.SliceStable(filteredProfiles, func(i, j int) bool {
		p1 := filteredProfiles[i]
		p2 := filteredProfiles[j]
		email1Lower := strings.ToLower(p1.Email)
		email2Lower := strings.ToLower(p2.Email)
		if email1Lower != email2Lower {
			return email1Lower < email2Lower // Primary sort: Email (case-insensitive)
		}
		return p1.ID < p2.ID // Secondary sort: ID (guaranteed unique)
	})


	// Paginate the results (using a similar helper as for documents, maybe move to utils?)
	startIndex := (page - 1) * limit
	endIndex := startIndex + limit

	if startIndex >= totalMatching {
		// Page out of bounds, return empty list
		c.JSON(http.StatusOK, gin.H{
			"data":  []models.Profile{},
			"total": totalMatching,
			"page":  page,
			"limit": limit,
		})
		return
	}

	if endIndex > totalMatching {
		endIndex = totalMatching
	}

	paginatedProfiles := filteredProfiles[startIndex:endIndex]

	// Return paginated list and total count using the defined struct
	c.JSON(http.StatusOK, SearchProfilesResponse{
		Data:  paginatedProfiles,
		Total: totalMatching,
		Page:  page,
		Limit: limit,
	})
}
