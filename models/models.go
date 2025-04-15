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
	PasswordHash   string    `json:"password_hash"`   // Store hash, include in JSON persistence.
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
	Mu sync.RWMutex `json:"-"` // Exclude mutex from serialization (Exported)

	// File path for persistence (obtained from configuration)
    filePath string `json:"-"`
    // Backup enabled flag (obtained from configuration)
    backupEnabled bool `json:"-"`
    // Save interval (obtained from configuration)
    saveInterval time.Duration `json:"-"`
}

// Add methods to Database for Load, Save, and CRUD operations on Profiles, Documents, ShareRecords, ensuring mutex usage.