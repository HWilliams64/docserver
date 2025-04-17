package db

import (
	"docserver/config"
	"docserver/models" // Corrected import path
	"docserver/utils"  // Added for GenerateDashlessUUID
	"encoding/json"
	"fmt"              // Added for errors
	"log"
	"os"
	"strings"          // Added for EqualFold
	"sync"
	"time"
)

// Database holds all application data and manages concurrent access
// We embed the models.Database struct to inherit its fields (Profiles, Documents, ShareRecords, mu)
// and add fields specific to the database *logic* (config, saveTimer, etc.)
type Database struct {
	models.Database // Embedded struct from models
	config          *config.Config
	saveTimer       *time.Timer   // Timer for debounced saving
	savePending     bool          // Flag to indicate if a save is queued
	saveMutex       sync.Mutex    // Mutex specifically for the save timer logic
	otpStore        map[string]otpRecord // Temporary store for password reset OTPs
	otpMutex        sync.Mutex    // Mutex for OTP store access
}

// otpRecord stores the OTP and its expiry time
type otpRecord struct {
	otp    string
	expiry time.Time
}

// NewDatabase creates and initializes a new Database instance.
// It loads the configuration and attempts to load existing data from the file.
func NewDatabase(cfg *config.Config) (*Database, error) {
	db := &Database{
		Database: models.Database{ // Initialize the embedded struct
			Profiles:     make(map[string]models.Profile),
			Documents:    make(map[string]models.Document),
			ShareRecords: make(map[string]models.ShareRecord),
			// mu is initialized automatically (zero value is usable)
		},
		config:   cfg,
		otpStore: make(map[string]otpRecord),
		// saveTimer, savePending, saveMutex, otpMutex are initialized automatically
	}

	// Assign config values needed by the embedded struct (if they were separate)
	// Since we are embedding, we might access cfg directly or store copies if needed
	// For now, we'll keep the config reference and access cfg.DbFilePath etc. directly in methods.

	log.Printf("INFO: Initializing database with file: %s", cfg.DbFilePath)
	err := db.Load()
	if err != nil {
		// Load handles logging specific errors (file not found vs. parse error)
		// If Load returns an error, it means it couldn't parse an existing file.
		// If the file just didn't exist, Load returns nil and initializes empty maps.
		// We only return the error if it's critical (e.g., parse error).
		if !os.IsNotExist(err) { // Check if the error is *not* 'file not found'
			log.Printf("ERROR: Database Load failed with critical error: %v", err)
			return nil, err // Propagate critical errors
		}
		// If the error was os.IsNotExist, Load already logged it and initialized empty maps, so we continue.
	} // Close the 'if err != nil' block

	return db, nil // Return db outside the error check
} // Close the NewDatabase function
		
		// Load reads the database state from the JSON file specified in the configuration.
		// If the file doesn't exist, it initializes an empty database state and logs a message.
		// If the file exists but cannot be parsed, it logs a critical error and returns it.
		func (db *Database) Load() error {
			// Access embedded fields explicitly
			db.Database.Mu.Lock() // Acquire write lock for loading (modifies the maps)
			defer db.Database.Mu.Unlock()
		
			fileData, err := os.ReadFile(db.config.DbFilePath)
			if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: Database file '%s' not found. Initializing empty database.", db.config.DbFilePath)
			// Ensure maps are initialized (already done in NewDatabase, but good practice here too)
			db.Database.Profiles = make(map[string]models.Profile)
			db.Database.Documents = make(map[string]models.Document)
			db.Database.ShareRecords = make(map[string]models.ShareRecord)
			return nil // Not an error if the file doesn't exist
		}
		// Other file read errors
		log.Printf("ERROR: Failed to read database file '%s': %v. Attempting to proceed with empty state.", db.config.DbFilePath, err)
		// Initialize empty maps just in case, although Load is usually called on an already initialized struct
		db.Database.Profiles = make(map[string]models.Profile)
		db.Database.Documents = make(map[string]models.Document)
		db.Database.ShareRecords = make(map[string]models.ShareRecord)
		// We might return the error here depending on desired strictness, but plan suggests continuing if possible.
		// Let's return nil for now, as the error is logged.
		return nil
	}

	// File exists, attempt to unmarshal
	// We unmarshal directly into the embedded models.Database part
	err = json.Unmarshal(fileData, &db.Database)
	if err != nil {
		log.Printf("CRITICAL: Failed to parse JSON data from database file '%s': %v. Server startup might be affected.", db.config.DbFilePath, err)
		// Do NOT wipe the in-memory state here if unmarshalling fails,
		// but ensure maps are initialized before returning the critical error.
		if db.Database.Profiles == nil {
			db.Database.Profiles = make(map[string]models.Profile)
		}
		if db.Database.Documents == nil {
			db.Database.Documents = make(map[string]models.Document)
		}
		if db.Database.ShareRecords == nil {
			db.Database.ShareRecords = make(map[string]models.ShareRecord)
		}
		// Return the error so the caller (NewDatabase) knows it's critical.
		return err
	}

	// Ensure maps are not nil after unmarshalling (if the JSON file had null values for them)
	if db.Database.Profiles == nil {
		db.Database.Profiles = make(map[string]models.Profile)
	}
	if db.Database.Documents == nil {
		db.Database.Documents = make(map[string]models.Document)
	}
	if db.Database.ShareRecords == nil {
		db.Database.ShareRecords = make(map[string]models.ShareRecord)
	}

	log.Printf("INFO: Successfully loaded database from %s. Profiles: %d, Documents: %d, ShareRecords: %d",
		db.config.DbFilePath, len(db.Database.Profiles), len(db.Database.Documents), len(db.Database.ShareRecords))

	return nil
}

// --- Placeholder for Save/Persist logic ---
// persist saves the current database state to the JSON file.
// This is the actual file writing logic, called by the debounced mechanism.
func (db *Database) persist() error {
	// Access embedded fields explicitly
	db.Database.Mu.RLock() // Use Read Lock for marshalling the current state
	defer db.Database.Mu.RUnlock()

	log.Printf("DEBUG: Persist triggered. Marshalling database state...")
	jsonData, err := json.MarshalIndent(&db.Database, "", "  ") // Use embedded struct
	if err != nil {
		log.Printf("ERROR: Failed to marshal database state to JSON: %v", err)
		return err // Don't proceed if marshalling fails
	}

	// --- Atomic Write ---
	tempFilePath := db.config.DbFilePath + ".tmp"
	backupFilePath := db.config.DbFilePath + ".bak"

	// Write to temporary file first
	err = os.WriteFile(tempFilePath, jsonData, 0644) // Sensible default permissions
	if err != nil {
		log.Printf("ERROR: Failed to write to temporary database file '%s': %v", tempFilePath, err)
		return err
	}

	// Handle backup if enabled
	if db.config.EnableBackup {
		// Check if original file exists before trying to back it up
		if _, err := os.Stat(db.config.DbFilePath); err == nil {
			// Original file exists, attempt rename to .bak
			err = os.Rename(db.config.DbFilePath, backupFilePath)
			if err != nil {
				// If rename fails (e.g., .bak exists and OS doesn't overwrite), log warning but continue
				log.Printf("WARN: Failed to rename '%s' to '%s' for backup: %v. Proceeding with save.", db.config.DbFilePath, backupFilePath, err)
				// Optionally, attempt to remove existing .bak first: os.Remove(backupFilePath)
			} else {
				log.Printf("DEBUG: Created backup file: %s", backupFilePath)
			}
		} else if !os.IsNotExist(err) {
            // Some other error occurred checking the original file status
            log.Printf("WARN: Error checking status of original DB file '%s' before backup: %v", db.config.DbFilePath, err)
        }
	}

	// Atomically rename temporary file to the final destination
	err = os.Rename(tempFilePath, db.config.DbFilePath)
	if err != nil {
		log.Printf("ERROR: Failed to atomically rename temporary file '%s' to '%s': %v", tempFilePath, db.config.DbFilePath, err)
		// Attempt to clean up the temp file
		_ = os.Remove(tempFilePath)
		// Consider trying to restore the backup if the rename failed? More complex.
		return err
	}

	log.Printf("INFO: Successfully saved database state to %s", db.config.DbFilePath)
	return nil
}


// --- Placeholder for Debounced Save logic ---
// requestSave is called after every write operation to trigger a debounced save.
func (db *Database) requestSave() {
    db.saveMutex.Lock() // Lock the save timer logic
    defer db.saveMutex.Unlock()

    // Instant save if interval is zero or negative
    if db.config.SaveInterval <= 0 {
        log.Printf("DEBUG: Save interval <= 0, triggering immediate persist.")
        // Run persist in a goroutine to avoid blocking the caller
        go func() {
            if err := db.persist(); err != nil {
                log.Printf("ERROR: Immediate persist failed: %v", err)
                // Implement retry logic here if needed
            }
        }()
        return
    }

    // Debounced save logic
    // If a timer is already running, stop it (reset the debounce period)
    if db.saveTimer != nil {
        db.saveTimer.Stop()
    }

    // Set the flag indicating a save is needed
    db.savePending = true

    // Start a new timer
    db.saveTimer = time.AfterFunc(db.config.SaveInterval, func() {
        db.saveMutex.Lock() // Lock for modifying savePending
        if !db.savePending {
            db.saveMutex.Unlock()
            return // Save was cancelled or already happened
        }
        db.savePending = false // Reset flag before starting persist
        db.saveMutex.Unlock()

        log.Printf("INFO: Debounced save interval elapsed. Persisting database...")
        if err := db.persist(); err != nil {
            log.Printf("ERROR: Debounced persist failed: %v", err)
            // Schedule a retry? For now, just log.
            // Could re-trigger requestSave() after a delay.
        }
    })
    log.Printf("DEBUG: Save requested. Debounce timer reset/started for %s.", db.config.SaveInterval)
}

// --- OTP Store Methods ---

// StoreOTP saves an OTP for a given email with an expiry time.
// It uses otpMutex for thread-safe access to the otpStore map.
func (db *Database) StoreOTP(email string, otp string, expiry time.Time) {
	db.otpMutex.Lock()
	defer db.otpMutex.Unlock()
	db.otpStore[email] = otpRecord{otp: otp, expiry: expiry}
	log.Printf("DEBUG: Stored OTP for %s", email)
}

// RetrieveOTP fetches the stored OTP and expiry time for a given email.
// It returns the otp, expiry time, and a boolean indicating if found.
// It uses otpMutex for thread-safe access.
func (db *Database) RetrieveOTP(email string) (string, time.Time, bool) {
	db.otpMutex.Lock() // Lock for reading and potential modification (if cleaning up expired)
	defer db.otpMutex.Unlock()

	record, found := db.otpStore[email]
	if !found {
		return "", time.Time{}, false
	}

	// Optional: Check for expiry here and delete if expired?
	// The VerifyOTP function in utils/auth.go already does this check,
	// but doing it here too could help keep the store clean proactively.
	// For now, we'll just return what we found.
	// if time.Now().After(record.expiry) {
	//     delete(db.otpStore, email)
	//     log.Printf("DEBUG: Cleaned up expired OTP for %s during retrieval", email)
	//     return "", time.Time{}, false
	// }

	return record.otp, record.expiry, true
}

// DeleteOTP removes the OTP record for a given email.
// It uses otpMutex for thread-safe access.
func (db *Database) DeleteOTP(email string) {
	db.otpMutex.Lock()
	defer db.otpMutex.Unlock()
	delete(db.otpStore, email)
	log.Printf("DEBUG: Deleted OTP for %s", email)
}


// --- CRUD Methods: Profiles ---

// CreateProfile adds a new profile to the database.
// It checks for email uniqueness.
// Returns the created profile or an error if the email already exists.
func (db *Database) CreateProfile(profile models.Profile) (models.Profile, error) {
	db.Database.Mu.Lock() // Full lock for checking uniqueness and writing
	defer db.Database.Mu.Unlock()

	// Check if email already exists (case-insensitive check recommended)
	for _, existingProfile := range db.Database.Profiles {
		if strings.EqualFold(existingProfile.Email, profile.Email) {
			return models.Profile{}, fmt.Errorf("email '%s' already exists", profile.Email)
		}
	}

	// Assign ID, timestamps if not already set (should be done by handler ideally)
	if profile.ID == "" {
		// Assuming utils.GenerateDashlessUUID exists and is imported
		// Need to add "docserver/utils" to imports
		profile.ID = utils.GenerateDashlessUUID()
	}
	now := time.Now().UTC()
	if profile.CreationDate.IsZero() {
		profile.CreationDate = now
	}
	profile.LastModifiedDate = now // Always update last modified on create/update

	db.Database.Profiles[profile.ID] = profile
	log.Printf("INFO: Created Profile ID: %s, Email: %s", profile.ID, profile.Email)

	// Trigger save
	db.requestSave()

	return profile, nil
}

// GetProfileByID retrieves a profile by its ID.
// Returns the profile and true if found, otherwise false.
func (db *Database) GetProfileByID(id string) (models.Profile, bool) {
	db.Database.Mu.RLock() // Read lock is sufficient
	defer db.Database.Mu.RUnlock()

	profile, found := db.Database.Profiles[id]
	return profile, found
}

// GetProfileByEmail retrieves a profile by its email address (case-insensitive).
// Returns the profile and true if found, otherwise false.
func (db *Database) GetProfileByEmail(email string) (models.Profile, bool) {
	db.Database.Mu.RLock() // Read lock
	defer db.Database.Mu.RUnlock()

	for _, profile := range db.Database.Profiles {
		if strings.EqualFold(profile.Email, email) {
			return profile, true
		}
	}
	return models.Profile{}, false
}

// UpdateProfile updates an existing profile.
// It finds the profile by ID and updates specified fields.
// Returns the updated profile or an error if not found.
// Note: This is a full update; partial updates (PATCH) require more logic.
func (db *Database) UpdateProfile(id string, updatedProfile models.Profile) (models.Profile, error) {
	db.Database.Mu.Lock() // Full lock for read-modify-write
	defer db.Database.Mu.Unlock()

	existingProfile, found := db.Database.Profiles[id]
	if !found {
		return models.Profile{}, fmt.Errorf("profile with ID '%s' not found", id)
	}

	// Preserve original creation date and ID
	updatedProfile.ID = existingProfile.ID
	updatedProfile.CreationDate = existingProfile.CreationDate
	updatedProfile.LastModifiedDate = time.Now().UTC() // Update modification timestamp
	// Ensure email isn't changed to one that already exists (unless it's the same profile)
	if !strings.EqualFold(existingProfile.Email, updatedProfile.Email) {
		for _, p := range db.Database.Profiles {
			if p.ID != id && strings.EqualFold(p.Email, updatedProfile.Email) {
				return models.Profile{}, fmt.Errorf("cannot update profile, email '%s' already exists for another user", updatedProfile.Email)
			}
		}
	}


	db.Database.Profiles[id] = updatedProfile
	log.Printf("INFO: Updated Profile ID: %s", id)

	// Trigger save
	db.requestSave()

	return updatedProfile, nil
}

// DeleteProfile removes a profile by its ID.
// Returns error if not found.
// Note: Also needs to handle associated data (documents, shares) later.
func (db *Database) DeleteProfile(id string) error {
	db.Database.Mu.Lock() // Full lock
	defer db.Database.Mu.Unlock()

	_, found := db.Database.Profiles[id]
	if !found {
		return fmt.Errorf("profile with ID '%s' not found", id)
	}

	delete(db.Database.Profiles, id)
	log.Printf("INFO: Deleted Profile ID: %s", id)

	// TODO: Implement cascading delete for documents owned by this profile
	// TODO: Implement removal from share records

	// Trigger save
	db.requestSave()

	return nil
}

// GetAllProfiles retrieves all profiles (potentially for searching/listing later).
// Consider adding filtering/pagination parameters here if needed directly in DB layer.
func (db *Database) GetAllProfiles() []models.Profile {
    db.Database.Mu.RLock()
    defer db.Database.Mu.RUnlock()

    profiles := make([]models.Profile, 0, len(db.Database.Profiles))
    for _, profile := range db.Database.Profiles {
        profiles = append(profiles, profile)
    }
    return profiles
}

// UpdateProfilePassword finds a profile by email and updates only its password hash.
// Returns error if the email is not found.
func (db *Database) UpdateProfilePassword(email string, newPasswordHash string) error {
 db.Database.Mu.Lock() // Full lock for read-modify-write
 defer db.Database.Mu.Unlock()

 var targetProfileID string
 found := false

 // Find the profile ID by email (case-insensitive)
 for id, profile := range db.Database.Profiles {
  if strings.EqualFold(profile.Email, email) {
   targetProfileID = id
   found = true
   break
  }
 }

 if !found {
  return fmt.Errorf("profile with email '%s' not found", email)
 }

 // Get the actual profile struct (must exist if found by email)
 profileToUpdate := db.Database.Profiles[targetProfileID]

 // Update hash and modification time
 profileToUpdate.PasswordHash = newPasswordHash
 profileToUpdate.LastModifiedDate = time.Now().UTC()

 // Save back to map
 db.Database.Profiles[targetProfileID] = profileToUpdate
 log.Printf("INFO: Updated password hash for Profile ID: %s (Email: %s)", targetProfileID, email)

 // Trigger save
 db.requestSave()

 return nil
}



// --- CRUD Methods: Documents ---

// CreateDocument adds a new document to the database.
func (db *Database) CreateDocument(doc models.Document) (models.Document, error) {
	db.Database.Mu.Lock()
	defer db.Database.Mu.Unlock()

	if doc.OwnerID == "" {
		// This should ideally be validated at the handler level
		return models.Document{}, fmt.Errorf("document must have an OwnerID")
	}
	// Check if owner profile exists? Optional, depends on desired strictness.
	// _, ownerExists := db.Database.Profiles[doc.OwnerID]
	// if !ownerExists {
	// 	 return models.Document{}, fmt.Errorf("owner profile with ID '%s' not found", doc.OwnerID)
	// }

	// Assign ID and timestamps
	doc.ID = utils.GenerateDashlessUUID()
	now := time.Now().UTC()
	doc.CreationDate = now
	doc.LastModifiedDate = now

	db.Database.Documents[doc.ID] = doc
	log.Printf("INFO: Created Document ID: %s, OwnerID: %s", doc.ID, doc.OwnerID)

	// Trigger save
	db.requestSave()

	return doc, nil
}

// GetDocumentByID retrieves a document by its ID.
func (db *Database) GetDocumentByID(id string) (models.Document, bool) {
	db.Database.Mu.RLock()
	defer db.Database.Mu.RUnlock()

	doc, found := db.Database.Documents[id]
	return doc, found
}

// GetDocumentsByOwner retrieves all documents owned by a specific profile ID.
// Note: This doesn't handle shared documents yet. Querying logic will combine this.
func (db *Database) GetDocumentsByOwner(ownerID string) []models.Document {
	db.Database.Mu.RLock()
	defer db.Database.Mu.RUnlock()

	docs := make([]models.Document, 0)
	for _, doc := range db.Database.Documents {
		if doc.OwnerID == ownerID {
			docs = append(docs, doc)
		}
	}
	return docs
}

// GetAllDocuments retrieves all documents. Used internally for filtering/querying.
func (db *Database) GetAllDocuments() []models.Document {
    db.Database.Mu.RLock()
    defer db.Database.Mu.RUnlock()

    docs := make([]models.Document, 0, len(db.Database.Documents))
    for _, doc := range db.Database.Documents {
        docs = append(docs, doc)
    }
    return docs
}


// UpdateDocument updates an existing document's content.
// Only the owner can update the document (checked at handler level).
func (db *Database) UpdateDocument(id string, newContent any) (models.Document, error) {
	db.Database.Mu.Lock()
	defer db.Database.Mu.Unlock()

	existingDoc, found := db.Database.Documents[id]
	if !found {
		return models.Document{}, fmt.Errorf("document with ID '%s' not found", id)
	}

	// Update content and timestamp
	existingDoc.Content = newContent
	existingDoc.LastModifiedDate = time.Now().UTC()

	db.Database.Documents[id] = existingDoc
	log.Printf("INFO: Updated Document ID: %s", id)

	// Trigger save
	db.requestSave()

	return existingDoc, nil
}

// DeleteDocument removes a document by its ID.
// Also removes the associated ShareRecord, if it exists.
// Only the owner can delete (checked at handler level).
func (db *Database) DeleteDocument(id string) error {
	db.Database.Mu.Lock()
	defer db.Database.Mu.Unlock()

	_, found := db.Database.Documents[id]
	if !found {
		return fmt.Errorf("document with ID '%s' not found", id)
	}

	// Delete the document
	delete(db.Database.Documents, id)
	log.Printf("INFO: Deleted Document ID: %s", id)

	// Also delete the corresponding share record
	_, shareRecordFound := db.Database.ShareRecords[id]
	if shareRecordFound {
		delete(db.Database.ShareRecords, id)
		log.Printf("INFO: Deleted associated ShareRecord for Document ID: %s", id)
	}

	// Trigger save
	db.requestSave()

	return nil
}


// --- CRUD Methods: ShareRecords ---

// GetShareRecordByDocumentID retrieves the share record for a specific document ID.
// Returns the record and true if found, otherwise false.
func (db *Database) GetShareRecordByDocumentID(docID string) (models.ShareRecord, bool) {
	db.Database.Mu.RLock()
	defer db.Database.Mu.RUnlock()

	record, found := db.Database.ShareRecords[docID]
	// Ensure the DocumentID field is set (it's the key, but good practice)
	if found {
		record.DocumentID = docID
	}
	return record, found
}

// SetShareRecord creates or replaces the entire share record for a document.
// It takes the document ID and a list of profile IDs (dashless) to share with.
// An empty or nil list effectively removes all shares.
func (db *Database) SetShareRecord(docID string, sharedWith []string) error {
	db.Database.Mu.Lock()
	defer db.Database.Mu.Unlock()

	// Optional: Check if document exists?
	// _, docFound := db.Database.Documents[docID]
	// if !docFound {
	// 	 return fmt.Errorf("cannot set share record for non-existent document ID '%s'", docID)
	// }

	// Optional: Validate that profile IDs in sharedWith actually exist? Can be slow.

	// Create or update the record
	// Ensure nil slices become empty slices for consistent JSON marshalling if needed
	if sharedWith == nil {
		sharedWith = []string{}
	}

	// Remove duplicates from sharedWith list before saving
	uniqueSharedWith := make([]string, 0, len(sharedWith))
	seen := make(map[string]struct{}, len(sharedWith))
	for _, profileID := range sharedWith {
		if _, ok := seen[profileID]; !ok {
			seen[profileID] = struct{}{}
			uniqueSharedWith = append(uniqueSharedWith, profileID)
		}
	}


	if len(uniqueSharedWith) > 0 {
		record := models.ShareRecord{
			DocumentID: docID, // Although not stored in JSON, useful internally
			SharedWith: uniqueSharedWith,
		}
		db.Database.ShareRecords[docID] = record
		log.Printf("INFO: Set/Updated ShareRecord for Document ID: %s, SharedWith: %d profiles", docID, len(uniqueSharedWith))
	} else {
		// If the list is empty, remove the share record entirely
		delete(db.Database.ShareRecords, docID)
		log.Printf("INFO: Removed ShareRecord for Document ID: %s (no sharers)", docID)
	}


	// Trigger save
	db.requestSave()

	return nil
}

// AddSharerToDocument adds a single profile ID to a document's share list.
// Returns error if document doesn't exist (optional check).
func (db *Database) AddSharerToDocument(docID, profileID string) error {
	db.Database.Mu.Lock()
	defer db.Database.Mu.Unlock()

	// Optional: Check if document exists?
	// Optional: Check if profile exists?

	record, found := db.Database.ShareRecords[docID]
	if !found {
		// No existing record, create a new one
		record = models.ShareRecord{
			DocumentID: docID,
			SharedWith: []string{profileID},
		}
	} else {
		// Check if profileID is already in the list
		alreadyShared := false
		for _, existingID := range record.SharedWith {
			if existingID == profileID {
				alreadyShared = true
				break
			}
		}
		if !alreadyShared {
			record.SharedWith = append(record.SharedWith, profileID)
		} else {
			// Already shared, no change needed
			return nil // Or return a specific indicator? For now, just return nil.
		}
	}

	db.Database.ShareRecords[docID] = record
	log.Printf("INFO: Added Sharer '%s' to Document ID: %s", profileID, docID)

	// Trigger save
	db.requestSave()

	return nil
}

// RemoveSharerFromDocument removes a single profile ID from a document's share list.
// If the list becomes empty, the entire share record for that document is removed.
// Returns error if document doesn't exist (optional check).
func (db *Database) RemoveSharerFromDocument(docID, profileID string) error {
	db.Database.Mu.Lock()
	defer db.Database.Mu.Unlock()

	record, found := db.Database.ShareRecords[docID]
	if !found {
		// No share record exists, nothing to remove
		return nil // Not an error, just nothing to do
	}

	// Find and remove the profileID from the list
	foundIndex := -1
	for i, existingID := range record.SharedWith {
		if existingID == profileID {
			foundIndex = i
			break
		}
	}

	if foundIndex != -1 {
		// Remove element by slicing
		record.SharedWith = append(record.SharedWith[:foundIndex], record.SharedWith[foundIndex+1:]...)

		if len(record.SharedWith) > 0 {
			// Update the record
			db.Database.ShareRecords[docID] = record
			log.Printf("INFO: Removed Sharer '%s' from Document ID: %s", profileID, docID)
		} else {
			// List is now empty, remove the whole record
			delete(db.Database.ShareRecords, docID)
			log.Printf("INFO: Removed ShareRecord for Document ID: %s (last sharer removed)", docID)
		}

		// Trigger save
		db.requestSave()

	} else {
		// Profile ID was not in the list, nothing to remove
		return nil
	}
return nil
}


// Close ensures any pending save operation is completed before shutdown.
func (db *Database) Close() error {
	var needsFinalPersist bool

	db.saveMutex.Lock()
	log.Printf("DEBUG: Closing database instance. Checking for pending save...")

	// Stop any active timer
	if db.saveTimer != nil {
		db.saveTimer.Stop()
		db.saveTimer = nil // Clear the timer
		log.Printf("DEBUG: Stopped active save timer.")
	}

	// Check if a save was pending *after* stopping the timer
	if db.savePending {
		needsFinalPersist = true
		db.savePending = false // Reset flag under lock
	}
	db.saveMutex.Unlock() // Release lock before potentially calling persist

	// Perform persist outside the lock if needed
	if needsFinalPersist {
		log.Printf("INFO: Performing final persist operation on close...")
		if err := db.persist(); err != nil {
			log.Printf("ERROR: Final persist operation failed during close: %v", err)
			return err // Return the error from persist
		}
		log.Printf("INFO: Final persist operation completed.")
	} else {
		log.Printf("DEBUG: No pending save operation on close.")
	}

	return nil
}