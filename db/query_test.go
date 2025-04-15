package db

import (
	"docserver/config" // Added
	"docserver/models" // Added
	"fmt" // Added
	"path/filepath" // Added for t.TempDir()
	"testing"
	"time" // Added

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson" // Added
)

// --- Parsing Tests ---

func TestParseSingleCondition(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectErr   bool
		expected    QueryCondition
		errContains string // Substring expected in error message
	}{
		{
			name:  "Valid: path operator value",
			input: `user.name equals "John Doe"`,
			expected: QueryCondition{
				Path: "user.name", Operator: "equals", ParsedValue: "John Doe", ValueType: gjson.String, IsInsensitive: false, Original: `user.name equals "John Doe"`,
			},
		},
		{
			name:  "Valid: operator value (root path)",
			input: `contains "keyword"`,
			expected: QueryCondition{
				Path: "", Operator: "contains", ParsedValue: "keyword", ValueType: gjson.String, IsInsensitive: false, Original: `contains "keyword"`,
			},
		},
		{
			name:  "Valid: numeric value",
			input: `age greaterThan 30`,
			expected: QueryCondition{
				Path: "age", Operator: "greaterthan", ParsedValue: float64(30), ValueType: gjson.Number, IsInsensitive: false, Original: `age greaterThan 30`,
			},
		},
		{
			name:  "Valid: boolean value",
			input: `isActive equals true`,
			expected: QueryCondition{
				Path: "isActive", Operator: "equals", ParsedValue: true, ValueType: gjson.True, IsInsensitive: false, Original: `isActive equals true`,
			},
		},
		{
			name:  "Valid: null value",
			input: `deletedAt equals null`,
			expected: QueryCondition{
				Path: "deletedAt", Operator: "equals", ParsedValue: nil, ValueType: gjson.Null, IsInsensitive: false, Original: `deletedAt equals null`,
			},
		},
		{
			name:  "Valid: value with spaces",
			input: `address.street contains "Main Street"`,
			expected: QueryCondition{
				Path: "address.street", Operator: "contains", ParsedValue: "Main Street", ValueType: gjson.String, IsInsensitive: false, Original: `address.street contains "Main Street"`,
			},
		},
		{
			name:  "Valid: case-insensitive operator",
			input: `tag equals-insensitive "urgent"`,
			expected: QueryCondition{
				Path: "tag", Operator: "equals", ParsedValue: "urgent", ValueType: gjson.String, IsInsensitive: true, Original: `tag equals-insensitive "urgent"`,
			},
		},
		{
			name:  "Valid: case-insensitive operator (root)",
			input: `contains-insensitive "important"`,
			expected: QueryCondition{
				Path: "", Operator: "contains", ParsedValue: "important", ValueType: gjson.String, IsInsensitive: true, Original: `contains-insensitive "important"`,
			},
		},
		{
			name:        "Invalid: too few parts (path operator)",
			input:       `user.name equals`,
			expectErr:   true,
			errContains: "condition must have at least an operator and a value",
		},
		{
			name:        "Invalid: too few parts (operator)",
			input:       `equals`,
			expectErr:   true,
			errContains: "condition must have at least an operator and a value",
		},
		{
			name:        "Invalid: too few parts (path)",
			input:       `user.name`,
			expectErr:   true,
			errContains: "condition must have at least an operator and a value",
		},
		{
			name:        "Invalid: operator",
			input:       `user.name invalidOp "value"`,
			expectErr:   true,
			errContains: "invalid operator 'invalidop'",
		},
		{
			name:        "Invalid: operator (root)",
			input:       `invalidOp "value"`,
			expectErr:   true,
			// This case is tricky, it might parse as path="invalidOp", operator="value", which is also invalid
			errContains: "invalid condition format", // Or "invalid operator 'value'" depending on parsing path
		},
		{
			name:        "Invalid: insensitive suffix on non-string op",
			input:       `age greaterThan-insensitive 30`,
			expectErr:   true,
			errContains: "invalid base operator for insensitive matching 'greaterthan'", // Base op is invalid for -insensitive
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseSingleCondition(tc.input)

			if tc.expectErr {
				require.Error(t, err, "Expected an error but got none")
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains, "Error message mismatch")
				}
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
				assert.Equal(t, tc.expected, result, "Parsed condition mismatch")
			}
		})
	}
}


func TestParseContentQuery(t *testing.T) {
	testCases := []struct {
		name        string
		input       []string
		expectErr   bool
		expected    *ParsedQuery
		errContains string
	}{
		{
			name:  "Valid: single condition",
			input: []string{`name equals "test"`},
			expected: &ParsedQuery{
				Conditions: []QueryCondition{
					{Path: "name", Operator: "equals", ParsedValue: "test", ValueType: gjson.String, IsInsensitive: false, Original: `name equals "test"`},
				},
				Logic: []LogicalOperator{},
			},
		},
		{
			name:  "Valid: two conditions with AND",
			input: []string{`name equals "test"`, "and", `age greaterThan 20`},
			expected: &ParsedQuery{
				Conditions: []QueryCondition{
					{Path: "name", Operator: "equals", ParsedValue: "test", ValueType: gjson.String, IsInsensitive: false, Original: `name equals "test"`},
					{Path: "age", Operator: "greaterthan", ParsedValue: float64(20), ValueType: gjson.Number, IsInsensitive: false, Original: `age greaterThan 20`},
				},
				Logic: []LogicalOperator{LogicAnd},
			},
		},
		{
			name:  "Valid: three conditions with OR and AND",
			input: []string{`status equals "active"`, "or", `tag contains "urgent"`, "and", `priority lessThan 5`},
			expected: &ParsedQuery{
				Conditions: []QueryCondition{
					// Note: "active" is not quoted in the input, so parser treats it as string
					{Path: "status", Operator: "equals", ParsedValue: "active", ValueType: gjson.String, IsInsensitive: false, Original: `status equals "active"`},
					// Note: "urgent" is not quoted in the input, so parser treats it as string
					{Path: "tag", Operator: "contains", ParsedValue: "urgent", ValueType: gjson.String, IsInsensitive: false, Original: `tag contains "urgent"`},
					{Path: "priority", Operator: "lessthan", ParsedValue: float64(5), ValueType: gjson.Number, IsInsensitive: false, Original: `priority lessThan 5`},
				},
				Logic: []LogicalOperator{LogicOr, LogicAnd},
			},
		},
		{
			name:  "Valid: empty input",
			input: []string{},
			expected: nil, // Expect nil for no query
		},
		{
			name:  "Valid: nil input",
			input: nil,
			expected: nil, // Expect nil for no query
		},
		{
			name:        "Invalid: starts with logic",
			input:       []string{"and", `name equals "test"`},
			expectErr:   true,
			errContains: "invalid condition at index 0", // Fails parsing condition
		},
		{
			name:        "Invalid: ends with logic",
			input:       []string{`name equals "test"`, "and"},
			expectErr:   true,
			errContains: "query must end with a condition",
		},
		{
			name:        "Invalid: consecutive conditions",
			input:       []string{`name equals "test"`, `age equals 30`},
			expectErr:   true,
			errContains: "invalid logical operator at index 1", // Expects logic, gets condition
		},
		{
			name:        "Invalid: consecutive logic",
			input:       []string{`name equals "test"`, "and", "or", `age equals 30`},
			expectErr:   true,
			errContains: "invalid condition at index 2", // Expects condition, gets logic
		},
		{
			name:        "Invalid: invalid logic operator",
			input:       []string{`name equals "test"`, "xor", `age equals 30`},
			expectErr:   true,
			errContains: "invalid logical operator at index 1: 'xor'",
		},
		{
			name:        "Invalid: empty part",
			input:       []string{`name equals "test"`, "and", ""},
			expectErr:   true,
			errContains: "query part at index 2 is empty",
		},
		{
			name:        "Invalid: condition parsing error bubbles up",
			input:       []string{`name equals "test"`, "and", `age greater`}, // Invalid condition
			expectErr:   true,
			errContains: "invalid condition at index 2", // Error from parseSingleCondition
			// errContains: "condition must have at least an operator and a value", // More specific check
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseContentQuery(tc.input)

			if tc.expectErr {
				require.Error(t, err, "Expected an error but got none")
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains, "Error message mismatch")
				}
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
				// Compare structs carefully, especially slices
				if tc.expected == nil {
					assert.Nil(t, result, "Expected nil result for empty query")
				} else {
					require.NotNil(t, result, "Expected non-nil result")
					assert.Equal(t, len(tc.expected.Conditions), len(result.Conditions), "Number of conditions mismatch")
					for i := range tc.expected.Conditions {
						assert.Equal(t, tc.expected.Conditions[i], result.Conditions[i], "Condition at index %d mismatch", i)
					}
					assert.Equal(t, len(tc.expected.Logic), len(result.Logic), "Number of logic operators mismatch")
					for i := range tc.expected.Logic {
						assert.Equal(t, tc.expected.Logic[i], result.Logic[i], "Logic operator at index %d mismatch", i)
					}
				}
			}
		})
	}
}

// --- Evaluation Tests ---

// Mock DB instance needed for EvaluateContentQuery method receiver
var testDBInstance *Database

func init() {
	// Create a minimal DB instance for tests that need the receiver method.
	// No actual file I/O or complex config needed for evaluation logic itself.
	cfg := &config.Config{} // Minimal config
	testDBInstance = &Database{
		Database: models.Database{ // Initialize embedded struct maps
			Profiles:     make(map[string]models.Profile),
			Documents:    make(map[string]models.Document),
			ShareRecords: make(map[string]models.ShareRecord),
		},
		config:   cfg,
		otpStore: make(map[string]otpRecord),
	}
}

// Test evaluateSingleCondition (covers compareJSONValue and comparePlainText)
func TestEvaluateSingleCondition(t *testing.T) {
	testCases := []struct {
		name        string
		docContent  interface{}
		condition   string // Condition string to parse
		expectMatch bool
		expectErr   bool
		errContains string
	}{
		// --- String Comparisons ---
		{name: "String equals: match", docContent: `{"name": "test"}`, condition: `name equals "test"`, expectMatch: true},
		{name: "String equals: no match", docContent: `{"name": "test"}`, condition: `name equals "other"`, expectMatch: false},
		{name: "String equals: root match", docContent: `"simple string"`, condition: `equals "simple string"`, expectMatch: true},
		{name: "String equals: root no match", docContent: `"simple string"`, condition: `equals "another string"`, expectMatch: false},
		{name: "String equals insensitive: match lower", docContent: `{"tag": "Urgent"}`, condition: `tag equals-insensitive "urgent"`, expectMatch: true},
		{name: "String equals insensitive: match upper", docContent: `{"tag": "urgent"}`, condition: `tag equals-insensitive "URGENT"`, expectMatch: true},
		{name: "String equals insensitive: no match", docContent: `{"tag": "urgent"}`, condition: `tag equals-insensitive "later"`, expectMatch: false},
		{name: "String notEquals: match", docContent: `{"name": "test"}`, condition: `name notEquals "other"`, expectMatch: true},
		{name: "String notEquals: no match", docContent: `{"name": "test"}`, condition: `name notEquals "test"`, expectMatch: false},
		{name: "String notEquals insensitive: match", docContent: `{"tag": "Urgent"}`, condition: `tag notequals-insensitive "later"`, expectMatch: true},
		{name: "String notEquals insensitive: no match", docContent: `{"tag": "Urgent"}`, condition: `tag notequals-insensitive "urgent"`, expectMatch: false},
		{name: "String contains: match", docContent: `{"desc": "hello world"}`, condition: `desc contains "llo wor"`, expectMatch: true},
		{name: "String contains: no match", docContent: `{"desc": "hello world"}`, condition: `desc contains "goodbye"`, expectMatch: false},
		{name: "String contains insensitive: match", docContent: `{"desc": "Hello World"}`, condition: `desc contains-insensitive "llo wor"`, expectMatch: true},
		{name: "String contains insensitive: no match", docContent: `{"desc": "Hello World"}`, condition: `desc contains-insensitive "goodbye"`, expectMatch: false},
		{name: "String startsWith: match", docContent: `{"file": "report.txt"}`, condition: `file startsWith "report"`, expectMatch: true},
		{name: "String startsWith: no match", docContent: `{"file": "report.txt"}`, condition: `file startsWith "txt"`, expectMatch: false},
		{name: "String startsWith insensitive: match", docContent: `{"file": "Report.txt"}`, condition: `file startswith-insensitive "report"`, expectMatch: true},
		{name: "String endsWith: match", docContent: `{"file": "report.txt"}`, condition: `file endsWith ".txt"`, expectMatch: true},
		{name: "String endsWith: no match", docContent: `{"file": "report.txt"}`, condition: `file endsWith "report"`, expectMatch: false},
		{name: "String endsWith insensitive: match", docContent: `{"file": "report.TXT"}`, condition: `file endswith-insensitive ".txt"`, expectMatch: true},
		{name: "String numeric op: error", docContent: `{"val": "100"}`, condition: `val greaterThan 50`, expectErr: true, errContains: "type mismatch: cannot apply numeric operator"},

		// --- Numeric Comparisons ---
		{name: "Number equals: match int", docContent: `{"age": 30}`, condition: `age equals 30`, expectMatch: true},
		{name: "Number equals: match float", docContent: `{"price": 19.99}`, condition: `price equals 19.99`, expectMatch: true},
		{name: "Number equals: no match", docContent: `{"age": 30}`, condition: `age equals 31`, expectMatch: false},
		{name: "Number notEquals: match", docContent: `{"age": 30}`, condition: `age notEquals 31`, expectMatch: true},
		{name: "Number notEquals: no match", docContent: `{"age": 30}`, condition: `age notEquals 30`, expectMatch: false},
		{name: "Number greaterThan: match", docContent: `{"score": 100}`, condition: `score greaterThan 90`, expectMatch: true},
		{name: "Number greaterThan: no match", docContent: `{"score": 100}`, condition: `score greaterThan 100`, expectMatch: false},
		{name: "Number lessThan: match", docContent: `{"temp": -5}`, condition: `temp lessThan 0`, expectMatch: true},
		{name: "Number lessThan: no match", docContent: `{"temp": -5}`, condition: `temp lessThan -5`, expectMatch: false},
		{name: "Number greaterThanOrEquals: match equal", docContent: `{"count": 5}`, condition: `count greaterThanOrEquals 5`, expectMatch: true},
		{name: "Number greaterThanOrEquals: match greater", docContent: `{"count": 6}`, condition: `count greaterThanOrEquals 5`, expectMatch: true},
		{name: "Number greaterThanOrEquals: no match", docContent: `{"count": 4}`, condition: `count greaterThanOrEquals 5`, expectMatch: false},
		{name: "Number lessThanOrEquals: match equal", docContent: `{"count": 5}`, condition: `count lessThanOrEquals 5`, expectMatch: true},
		{name: "Number lessThanOrEquals: match less", docContent: `{"count": 4}`, condition: `count lessThanOrEquals 5`, expectMatch: true},
		{name: "Number lessThanOrEquals: no match", docContent: `{"count": 6}`, condition: `count lessThanOrEquals 5`, expectMatch: false},
		{name: "Number invalid value: error", docContent: `{"age": 30}`, condition: `age equals thirty`, expectErr: true, errContains: "type mismatch: value 'thirty' is not a valid number"},
		{name: "Number string op: error", docContent: `{"age": 30}`, condition: `age contains 3`, expectErr: true, errContains: "type mismatch: cannot apply string operator"},
		{name: "Number insensitive op: error", docContent: `{"age": 30}`, condition: `age equals-insensitive 30`, expectErr: true, errContains: "cannot be case-insensitive for numeric comparison"},

		// --- Boolean Comparisons ---
		{name: "Boolean equals: true match", docContent: `{"isActive": true}`, condition: `isActive equals true`, expectMatch: true},
		{name: "Boolean equals: false match", docContent: `{"isActive": false}`, condition: `isActive equals false`, expectMatch: true},
		{name: "Boolean equals: no match", docContent: `{"isActive": true}`, condition: `isActive equals false`, expectMatch: false},
		{name: "Boolean notEquals: match", docContent: `{"isActive": true}`, condition: `isActive notEquals false`, expectMatch: true},
		{name: "Boolean notEquals: no match", docContent: `{"isActive": true}`, condition: `isActive notEquals true`, expectMatch: false},
		{name: "Boolean invalid value: error", docContent: `{"isActive": true}`, condition: `isActive equals yes`, expectErr: true, errContains: "type mismatch: value 'yes' is not a valid boolean"},
		{name: "Boolean invalid op: error", docContent: `{"isActive": true}`, condition: `isActive greaterThan false`, expectErr: true, errContains: "operator 'greaterthan' is invalid for boolean comparison"},
		{name: "Boolean insensitive op: error", docContent: `{"isActive": true}`, condition: `isActive equals-insensitive true`, expectErr: true, errContains: "cannot be case-insensitive for boolean comparison"},

		// --- Null Comparisons ---
		{name: "Null equals: match", docContent: `{"deletedAt": null}`, condition: `deletedAt equals null`, expectMatch: true},
		{name: "Null equals: no match (value not null)", docContent: `{"deletedAt": "2024-01-01"}`, condition: `deletedAt equals null`, expectMatch: false},
		{name: "Null equals: no match (target not null)", docContent: `{"deletedAt": null}`, condition: `deletedAt equals "not null"`, expectMatch: false},
		{name: "Null notEquals: match (value not null)", docContent: `{"deletedAt": "2024-01-01"}`, condition: `deletedAt notEquals null`, expectMatch: true},
		{name: "Null notEquals: match (target not null)", docContent: `{"deletedAt": null}`, condition: `deletedAt notEquals "not null"`, expectMatch: true},
		{name: "Null notEquals: no match", docContent: `{"deletedAt": null}`, condition: `deletedAt notEquals null`, expectMatch: false},
		{name: "Null invalid op: error", docContent: `{"deletedAt": null}`, condition: `deletedAt greaterThan null`, expectErr: true, errContains: "operator 'greaterthan' invalid for null comparison"},

		// --- Array Comparisons (contains) ---
		{name: "Array contains string: match", docContent: `{"tags": ["A", "B", "C"]}`, condition: `tags contains "B"`, expectMatch: true},
		{name: "Array contains string: no match", docContent: `{"tags": ["A", "B", "C"]}`, condition: `tags contains "D"`, expectMatch: false},
		{name: "Array contains string insensitive: match", docContent: `{"tags": ["apple", "Banana"]}`, condition: `tags contains-insensitive "banana"`, expectMatch: true},
		{name: "Array contains number: match", docContent: `{"scores": [10, 20, 30]}`, condition: `scores contains 20`, expectMatch: true},
		{name: "Array contains number: no match", docContent: `{"scores": [10, 20, 30]}`, condition: `scores contains 25`, expectMatch: false},
		{name: "Array contains boolean: match", docContent: `{"flags": [true, false]}`, condition: `flags contains false`, expectMatch: true},
		{name: "Array contains boolean: no match", docContent: `{"flags": [true]}`, condition: `flags contains false`, expectMatch: false},
		{name: "Array contains null: match", docContent: `{"values": [1, null, "a"]}`, condition: `values contains null`, expectMatch: true},
		{name: "Array contains null: no match", docContent: `{"values": [1, "a"]}`, condition: `values contains null`, expectMatch: false},
		{name: "Array contains: type mismatch in value", docContent: `{"scores": [10, 20]}`, condition: `scores contains "10"`, expectMatch: false}, // String "10" != Number 10
		{name: "Array invalid op: error", docContent: `{"tags": ["A", "B"]}`, condition: `tags equals "A"`, expectErr: true, errContains: "operator 'equals' cannot directly compare arrays/objects"},
		{name: "Array invalid op: error gt", docContent: `{"tags": ["A", "B"]}`, condition: `tags greaterThan "A"`, expectErr: true, errContains: "operator 'greaterthan' is invalid for array comparison"},

		// --- Object Comparisons ---
		{name: "Object invalid op: error", docContent: `{"user": {"name": "X"}}`, condition: `user equals {}`, expectErr: true, errContains: "operator 'equals' cannot directly compare JSON objects"},

		// --- Path Issues ---
		{name: "Path non-existent: error", docContent: `{"name": "test"}`, condition: `address.street equals "Main"`, expectErr: true, errContains: "path 'address.street' does not exist"},
		{name: "Path non-existent (root): ok", docContent: `{}`, condition: `equals "test"`, expectMatch: false}, // Root exists but is empty object

		// --- Plain Text Content ---
		{name: "Plain text equals: match", docContent: `Just simple text.`, condition: `equals "Just simple text."`, expectMatch: true},
		{name: "Plain text equals: no match", docContent: `Simple text`, condition: `equals "Other text"`, expectMatch: false},
		{name: "Plain text contains: match", docContent: `Some important notice.`, condition: `contains "important"`, expectMatch: true},
		{name: "Plain text contains insensitive: match", docContent: `Some IMPORTANT notice.`, condition: `contains-insensitive "important"`, expectMatch: true},
		{name: "Plain text startsWith: match", docContent: `START middle end`, condition: `startsWith "START"`, expectMatch: true},
		{name: "Plain text endsWith: match", docContent: `START middle end`, condition: `endsWith "end"`, expectMatch: true},
		{name: "Plain text invalid op: error", docContent: `12345`, condition: `greaterThan 100`, expectErr: true, errContains: "content is plain text, and operator 'greaterthan' is not supported"},
		{name: "Plain text invalid op (root): error", docContent: `12345`, condition: `greaterThan 100`, expectErr: true, errContains: "content is plain text, and operator 'greaterthan' is not supported"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			doc := models.Document{ID: "doc1", OwnerID: "owner1", Content: tc.docContent}
			cond, err := parseSingleCondition(tc.condition)
			require.NoError(t, err, "Failed to parse test condition: %s", tc.condition)

			match, err := testDBInstance.evaluateSingleCondition(doc, cond)

			if tc.expectErr {
				require.Error(t, err, "Expected an error but got none")
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains, "Error message mismatch")
				}
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
				assert.Equal(t, tc.expectMatch, match, "Match result mismatch")
			}
		})
	}
}


// Test EvaluateContentQuery (covers logic combination)
func TestEvaluateContentQuery(t *testing.T) {
	doc1 := models.Document{ID: "doc1", Content: `{"name": "test", "age": 30, "tags": ["A", "B"]}`}
	doc2 := models.Document{ID: "doc2", Content: `{"name": "another", "age": 25, "tags": ["B", "C"]}`}
	doc3 := models.Document{ID: "doc3", Content: `{"name": "test", "age": 40, "tags": ["C", "D"]}`}

	testCases := []struct {
		name        string
		doc         models.Document
		queryParts  []string
		expectMatch bool
		expectErr   bool
		errContains string
	}{
		{name: "Single condition: match", doc: doc1, queryParts: []string{`name equals "test"`}, expectMatch: true},
		{name: "Single condition: no match", doc: doc1, queryParts: []string{`name equals "wrong"`}, expectMatch: false},
		{name: "AND: both match", doc: doc1, queryParts: []string{`name equals "test"`, "and", `age equals 30`}, expectMatch: true},
		{name: "AND: first no match", doc: doc1, queryParts: []string{`name equals "wrong"`, "and", `age equals 30`}, expectMatch: false},
		{name: "AND: second no match", doc: doc1, queryParts: []string{`name equals "test"`, "and", `age equals 31`}, expectMatch: false},
		{name: "AND: both no match", doc: doc1, queryParts: []string{`name equals "wrong"`, "and", `age equals 31`}, expectMatch: false},
		{name: "OR: first match", doc: doc2, queryParts: []string{`name equals "another"`, "or", `age equals 30`}, expectMatch: true},
		{name: "OR: second match", doc: doc2, queryParts: []string{`name equals "wrong"`, "or", `age equals 25`}, expectMatch: true},
		{name: "OR: both match", doc: doc2, queryParts: []string{`name equals "another"`, "or", `age equals 25`}, expectMatch: true},
		{name: "OR: neither match", doc: doc2, queryParts: []string{`name equals "wrong"`, "or", `age equals 30`}, expectMatch: false},
		{name: "Complex: (name=test AND age>35) OR tags contains B", doc: doc1, queryParts: []string{`name equals "test"`, "and", `age greaterThan 35`, "or", `tags contains "B"`}, expectMatch: true}, // (F and F) or T = T
		{name: "Complex: (name=test AND age>35) OR tags contains B", doc: doc2, queryParts: []string{`name equals "test"`, "and", `age greaterThan 35`, "or", `tags contains "B"`}, expectMatch: true}, // (F and F) or T = T
		{name: "Complex: (name=test AND age>35) OR tags contains B", doc: doc3, queryParts: []string{`name equals "test"`, "and", `age greaterThan 35`, "or", `tags contains "B"`}, expectMatch: true}, // (T and T) or F = T
		{name: "Empty query: match", doc: doc1, queryParts: []string{}, expectMatch: true},
		{name: "Nil query: match", doc: doc1, queryParts: nil, expectMatch: true},
		{name: "Evaluation error: bubbles up", doc: doc1, queryParts: []string{`name equals "test"`, "and", `nonexistent greaterThan 10`}, expectErr: true, errContains: "path 'nonexistent' does not exist"},
		{name: "Parsing error: bubbles up", doc: doc1, queryParts: []string{`name equals "test"`, "and", `age greater`}, expectErr: true, errContains: "invalid content_query"}, // Error comes from ParseContentQuery
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedQuery, parseErr := ParseContentQuery(tc.queryParts)
			// Handle expected parsing errors separately
			if tc.errContains == "invalid content_query" {
				require.Error(t, parseErr, "Expected parsing error")
				assert.Contains(t, parseErr.Error(), "invalid condition", "Underlying parsing error mismatch")
				return // Don't proceed to evaluation if parsing failed as expected
			}
			require.NoError(t, parseErr, "Parsing failed unexpectedly: %v", parseErr)


			match, evalErr := testDBInstance.EvaluateContentQuery(tc.doc, parsedQuery)

			if tc.expectErr {
				require.Error(t, evalErr, "Expected an evaluation error but got none")
				if tc.errContains != "" {
					assert.Contains(t, evalErr.Error(), tc.errContains, "Error message mismatch")
				}
			} else {
				require.NoError(t, evalErr, "Expected no evaluation error but got: %v", evalErr)
				assert.Equal(t, tc.expectMatch, match, "Match result mismatch")
			}
		})
	}
}


// --- Sorting Tests ---

func TestSortDocuments(t *testing.T) {
	// Create some sample docs with different timestamps
	time1 := time.Now().UTC()
	time2 := time1.Add(time.Minute)
	time3 := time1.Add(-time.Minute) // Earlier

	doc1 := models.Document{ID: "doc1", CreationDate: time1, LastModifiedDate: time3} // Created first, modified earliest
	doc2 := models.Document{ID: "doc2", CreationDate: time2, LastModifiedDate: time2} // Created last, modified last
	doc3 := models.Document{ID: "doc3", CreationDate: time3, LastModifiedDate: time1} // Created earliest, modified middle

	testCases := []struct {
		name        string
		sortBy      string
		order       string
		inputDocs   []models.Document
		expectedIDs []string // Expected order of IDs after sorting
		expectErr   bool
		errContains string
	}{
		// Creation Date Sorting
		{name: "Sort by creation_date asc", sortBy: "creation_date", order: "asc",
			inputDocs:   []models.Document{doc1, doc2, doc3},
			expectedIDs: []string{"doc3", "doc1", "doc2"}, // time3, time1, time2
		},
		{name: "Sort by creation_date desc", sortBy: "creation_date", order: "desc",
			inputDocs:   []models.Document{doc1, doc2, doc3},
			expectedIDs: []string{"doc2", "doc1", "doc3"}, // time2, time1, time3
		},
		{name: "Sort by creation_date default order (asc)", sortBy: "creation_date", order: "",
			inputDocs:   []models.Document{doc1, doc2, doc3},
			expectedIDs: []string{"doc3", "doc1", "doc2"},
		},
		{name: "Sort by default field (creation_date) asc", sortBy: "", order: "asc",
			inputDocs:   []models.Document{doc1, doc2, doc3},
			expectedIDs: []string{"doc3", "doc1", "doc2"},
		},
		{name: "Sort by default field (creation_date) desc", sortBy: "", order: "desc",
			inputDocs:   []models.Document{doc1, doc2, doc3},
			expectedIDs: []string{"doc2", "doc1", "doc3"},
		},

		// Last Modified Date Sorting
		{name: "Sort by last_modified_date asc", sortBy: "last_modified_date", order: "asc",
			inputDocs:   []models.Document{doc1, doc2, doc3},
			expectedIDs: []string{"doc1", "doc3", "doc2"}, // time3, time1, time2
		},
		{name: "Sort by last_modified_date desc", sortBy: "last_modified_date", order: "desc",
			inputDocs:   []models.Document{doc1, doc2, doc3},
			expectedIDs: []string{"doc2", "doc3", "doc1"}, // time2, time1, time3
		},

		// Edge Cases
		{name: "Sort empty list", sortBy: "creation_date", order: "asc",
			inputDocs:   []models.Document{},
			expectedIDs: []string{},
		},
		{name: "Sort single item list", sortBy: "creation_date", order: "asc",
			inputDocs:   []models.Document{doc1},
			expectedIDs: []string{"doc1"},
		},

		// Error Cases
		{name: "Invalid sortBy field", sortBy: "invalid_field", order: "asc",
			inputDocs:   []models.Document{doc1, doc2},
			expectErr:   true,
			errContains: "invalid sort_by value: 'invalid_field'",
		},
		{name: "Invalid order value", sortBy: "creation_date", order: "ascending",
			inputDocs:   []models.Document{doc1, doc2},
			expectErr:   true,
			errContains: "invalid order value: 'ascending'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a copy to avoid modifying the slice between tests
			docsToSort := make([]models.Document, len(tc.inputDocs))
			copy(docsToSort, tc.inputDocs)

			err := sortDocuments(docsToSort, tc.sortBy, tc.order)

			if tc.expectErr {
				require.Error(t, err, "Expected an error but got none")
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains, "Error message mismatch")
				}
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
				// Extract IDs from sorted docs
				resultIDs := make([]string, len(docsToSort))
				for i, doc := range docsToSort {
					resultIDs[i] = doc.ID
				}
				assert.Equal(t, tc.expectedIDs, resultIDs, "Sorted order mismatch")
			}
		})
	}
}


// --- Pagination Tests ---

func TestPaginateDocuments(t *testing.T) {
	// Create a larger list of dummy documents
	docs := make([]models.Document, 0, 15)
	for i := 0; i < 15; i++ {
		docs = append(docs, models.Document{ID: fmt.Sprintf("doc%d", i+1)})
	} // IDs: doc1, doc2, ..., doc15

	testCases := []struct {
		name        string
		page        int
		limit       int
		inputDocs   []models.Document
		expectedIDs []string // Expected IDs in the result slice
		expectErr   bool
		errContains string
	}{
		// Basic Pagination
		{name: "Page 1, Limit 5", page: 1, limit: 5, inputDocs: docs, expectedIDs: []string{"doc1", "doc2", "doc3", "doc4", "doc5"}},
		{name: "Page 2, Limit 5", page: 2, limit: 5, inputDocs: docs, expectedIDs: []string{"doc6", "doc7", "doc8", "doc9", "doc10"}},
		{name: "Page 3, Limit 5 (partial)", page: 3, limit: 5, inputDocs: docs, expectedIDs: []string{"doc11", "doc12", "doc13", "doc14", "doc15"}},

		// Edge Cases: Page/Limit Values
		{name: "Page 0 (defaults to 1)", page: 0, limit: 3, inputDocs: docs, expectedIDs: []string{"doc1", "doc2", "doc3"}},
		{name: "Page -1 (defaults to 1)", page: -1, limit: 3, inputDocs: docs, expectedIDs: []string{"doc1", "doc2", "doc3"}},
		// Expect all docs since defaultLimit (20) > len(docs) (15)
		{name: "Limit 0 (defaults to defaultLimit)", page: 1, limit: 0, inputDocs: docs, expectedIDs: []string{"doc1", "doc2", "doc3", "doc4", "doc5", "doc6", "doc7", "doc8", "doc9", "doc10", "doc11", "doc12", "doc13", "doc14", "doc15"}},
		{name: "Limit -1 (defaults to defaultLimit)", page: 1, limit: -1, inputDocs: docs, expectedIDs: []string{"doc1", "doc2", "doc3", "doc4", "doc5", "doc6", "doc7", "doc8", "doc9", "doc10", "doc11", "doc12", "doc13", "doc14", "doc15"}},
		// Expect all docs since maxLimit (100) > len(docs) (15)
		{name: "Limit > maxLimit (caps at maxLimit)", page: 1, limit: maxLimit + 10, inputDocs: docs, expectedIDs: []string{"doc1", "doc2", "doc3", "doc4", "doc5", "doc6", "doc7", "doc8", "doc9", "doc10", "doc11", "doc12", "doc13", "doc14", "doc15"}},

		// Edge Cases: Boundaries
		{name: "Page out of bounds (high)", page: 4, limit: 5, inputDocs: docs, expectedIDs: []string{}}, // Page 4 with limit 5 is empty
		{name: "Page exactly at end", page: 3, limit: 5, inputDocs: docs, expectedIDs: []string{"doc11", "doc12", "doc13", "doc14", "doc15"}},
		{name: "Limit larger than total items", page: 1, limit: 20, inputDocs: docs, expectedIDs: []string{"doc1", "doc2", "doc3", "doc4", "doc5", "doc6", "doc7", "doc8", "doc9", "doc10", "doc11", "doc12", "doc13", "doc14", "doc15"}},

		// Edge Cases: Input Slice
		{name: "Empty input slice", page: 1, limit: 5, inputDocs: []models.Document{}, expectedIDs: []string{}},
		{name: "Nil input slice", page: 1, limit: 5, inputDocs: nil, expectedIDs: []string{}}, // Should handle nil gracefully

		// Error Cases (Currently none defined in paginateDocuments, but could add if needed)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Note: paginateDocuments doesn't modify the input slice, so no copy needed
			resultDocs, err := paginateDocuments(tc.inputDocs, tc.page, tc.limit)

			if tc.expectErr {
				require.Error(t, err, "Expected an error but got none")
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains, "Error message mismatch")
				}
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
				// Extract IDs from result docs
				resultIDs := make([]string, len(resultDocs))
				for i, doc := range resultDocs {
					resultIDs[i] = doc.ID
				}
				assert.Equal(t, tc.expectedIDs, resultIDs, "Paginated result mismatch")
			}
		})
	}
}

// --- QueryDocuments Integration Tests ---

func TestQueryDocuments(t *testing.T) {
	// --- Test Setup ---
	// Create a temporary directory for the test database file
	tempDir := t.TempDir()
	tempDbPath := filepath.Join(tempDir, "test_query_docs.json")

	cfg := &config.Config{
		JwtSecret:    "test-secret",
		DbFilePath:   tempDbPath, // Provide a valid path for saving
		SaveInterval: -1,         // Disable debounced saving for tests, use immediate persist
	}
	db, err := NewDatabase(cfg)
	require.NoError(t, err, "Setup: Failed to create test database instance")

	// Clear existing data (important for isolated tests)
	db.Database.Mu.Lock() // Lock the embedded mutex
	db.Database.Profiles = make(map[string]models.Profile)
	db.Database.Documents = make(map[string]models.Document)
	db.Database.ShareRecords = make(map[string]models.ShareRecord)
	db.Database.Mu.Unlock() // Unlock the embedded mutex

	// Users
	user1ID := "user1"
	user2ID := "user2"
	// user3ID := "user3" // Removed as unused for now

	// Timestamps (ensure slight difference for stable sorting)
	time1 := time.Now().UTC().Add(-2 * time.Hour)
	time1a := time1.Add(time.Second) // Slightly after time1
	time2 := time.Now().UTC().Add(-1 * time.Hour)
	time3 := time.Now().UTC()

	// Documents
	doc1 := models.Document{ID: "doc1", OwnerID: user1ID, Content: `{"name": "alpha", "value": 10}`, CreationDate: time1, LastModifiedDate: time1}
	doc2 := models.Document{ID: "doc2", OwnerID: user1ID, Content: `{"name": "beta", "value": 20}`, CreationDate: time2, LastModifiedDate: time3} // Modified later
	doc3 := models.Document{ID: "doc3", OwnerID: user2ID, Content: `{"name": "gamma", "value": 15}`, CreationDate: time3, LastModifiedDate: time3}
	doc4 := models.Document{ID: "doc4", OwnerID: user1ID, Content: `Just plain text`, CreationDate: time1a, LastModifiedDate: time2} // Use time1a

	// Add documents directly to the map with fixed IDs
	db.Database.Mu.Lock()
	db.Database.Documents["doc1"] = doc1
	db.Database.Documents["doc2"] = doc2
	db.Database.Documents["doc3"] = doc3
	db.Database.Documents["doc4"] = doc4
	db.Database.Mu.Unlock()

	// Sharing: Share doc2 (owned by user1) with user2
	err = db.SetShareRecord(doc2.ID, []string{user2ID}) // SetShareRecord only returns an error
	require.NoError(t, err, "Setup: Failed to set share record")

	// --- Test Cases ---
	testCases := []struct {
		name           string
		params         QueryDocumentsParams
		expectedIDs    []string // Expected IDs in the *paginated* result
		expectedTotal  int      // Expected *total* matching count (before pagination)
		expectErr      bool
		errContains    string
	}{
		// --- Scope Filtering ---
		{
			name:          "Scope: owned by user1",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "owned"},
			expectedIDs:   []string{"doc1", "doc4", "doc2"}, // Default sort: creation_date asc (time1, time1a, time2)
			expectedTotal: 3,
		},
		{
			name:          "Scope: owned by user2",
			params:        QueryDocumentsParams{AuthUserID: user2ID, Scope: "owned"},
			expectedIDs:   []string{"doc3"},
			expectedTotal: 1,
		},
		{
			name:          "Scope: shared with user2",
			params:        QueryDocumentsParams{AuthUserID: user2ID, Scope: "shared"},
			expectedIDs:   []string{"doc2"}, // Only doc2 is shared with user2
			expectedTotal: 1,
		},
		{
			name:          "Scope: shared with user1 (none)",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "shared"},
			expectedIDs:   []string{},
			expectedTotal: 0,
		},
		{
			name:          "Scope: all for user1 (owned)",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "all"},
			expectedIDs:   []string{"doc1", "doc4", "doc2"}, // Default sort: creation_date asc (time1, time1a, time2)
			expectedTotal: 3,
		},
		{
			name:          "Scope: all for user2 (owned + shared)",
			params:        QueryDocumentsParams{AuthUserID: user2ID, Scope: "all"},
			expectedIDs:   []string{"doc2", "doc3"}, // Default sort: creation_date asc (time2, time3)
			expectedTotal: 2,
		},
		{
			name:          "Scope: default (empty) for user2 (equals all)",
			params:        QueryDocumentsParams{AuthUserID: user2ID, Scope: ""},
			expectedIDs:   []string{"doc2", "doc3"}, // Default sort: creation_date asc (time2, time3)
			expectedTotal: 2,
		},
		{
			name:          "Scope: invalid scope",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "invalid"},
			expectErr:     true,
			errContains:   "invalid scope value: 'invalid'",
		},

		// --- Content Filtering (with Scope) ---
		{
			name:          "Scope owned, Content match",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "owned", ContentQuery: []string{`name equals "beta"`}},
			expectedIDs:   []string{"doc2"},
			expectedTotal: 1,
		},
		{
			name:          "Scope owned, Content no match",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "owned", ContentQuery: []string{`name equals "gamma"`}},
			expectedIDs:   []string{},
			expectedTotal: 0,
		},
		{
			name:          "Scope shared, Content match",
			params:        QueryDocumentsParams{AuthUserID: user2ID, Scope: "shared", ContentQuery: []string{`value greaterThan 15`}},
			expectedIDs:   []string{"doc2"}, // doc2 (value 20) is shared and matches
			expectedTotal: 1,
		},
		{
			name:          "Scope all, Content match owned and shared",
			params:        QueryDocumentsParams{AuthUserID: user2ID, Scope: "all", ContentQuery: []string{`value greaterThan 10`}},
			expectedIDs:   []string{"doc2", "doc3"}, // doc2 (shared, 20), doc3 (owned, 15) match. Default sort creation_date asc (time2, time3).
			expectedTotal: 2,
		},
		{
			name:          "Scope all, Content match plain text",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "all", ContentQuery: []string{`contains "plain"`}},
			expectedIDs:   []string{"doc4"}, // Only doc4 should match
			expectedTotal: 1,
		},
		{
			name:          "Scope all, Invalid content query",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "all", ContentQuery: []string{`name equals`}},
			expectErr:     true,
			errContains:   "invalid content_query",
		},
		{
			name:          "Scope all, Content evaluation error (should skip doc, not fail query)",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "all", ContentQuery: []string{`nonexistent equals "a"`}},
			expectedIDs:   []string{}, // No docs should match this invalid path query
			expectedTotal: 0,          // Expect 0 matches, not an error from QueryDocuments
			expectErr:     false,      // QueryDocuments itself shouldn't error here
		},


		// --- Sorting (with Scope/Content) ---
		{
			name:          "Sort by last_modified_date desc",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "owned", SortBy: "last_modified_date", Order: "desc"},
			expectedIDs:   []string{"doc2", "doc4", "doc1"}, // doc2(time3), doc4(time2), doc1(time1)
			expectedTotal: 3,
		},
		{
			name:          "Sort by creation_date desc (content query)",
			params:        QueryDocumentsParams{AuthUserID: user2ID, Scope: "all", ContentQuery: []string{`value greaterThan 10`}, SortBy: "creation_date", Order: "desc"},
			expectedIDs:   []string{"doc3", "doc2"}, // doc3 (time3), doc2 (time2) match query
			expectedTotal: 2,
		},
		{
			name:          "Invalid sortBy",
			params:        QueryDocumentsParams{AuthUserID: user1ID, SortBy: "invalid"},
			expectErr:     true,
			errContains:   "invalid sort_by value: 'invalid'",
		},
		{
			name:          "Invalid order",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Order: "invalid"},
			expectErr:     true,
			errContains:   "invalid order value: 'invalid'",
		},

		// --- Pagination (with Scope/Content/Sort) ---
		{
			name:          "Paginate owned user1 (page 1, limit 2)",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "owned", Page: 1, Limit: 2}, // Default sort creation_date asc
			expectedIDs:   []string{"doc1", "doc4"}, // time1, time1a
			expectedTotal: 3,                        // Total owned by user1
		},
		{
			name:          "Paginate owned user1 (page 2, limit 2)",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "owned", Page: 2, Limit: 2}, // Default sort creation_date asc
			expectedIDs:   []string{"doc2"},
			expectedTotal: 3,
		},
		{
			name:          "Paginate owned user1 (page 3, limit 2 - empty)",
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "owned", Page: 3, Limit: 2},
			expectedIDs:   []string{},
			expectedTotal: 3,
		},
		{
			name:          "Paginate with default limit", // defaultLimit is 20
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "owned", Page: 1, Limit: 0},
			expectedIDs:   []string{"doc1", "doc4", "doc2"}, // Default sort: time1, time1a, time2
			expectedTotal: 3,
		},
		{
			name:          "Paginate with max limit", // maxLimit is 100
			params:        QueryDocumentsParams{AuthUserID: user1ID, Scope: "owned", Page: 1, Limit: maxLimit + 1},
			expectedIDs:   []string{"doc1", "doc4", "doc2"}, // Default sort: time1, time1a, time2
			expectedTotal: 3,
		},
	}

	// --- Run Test Cases ---
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure default limit/page are handled if not set in test case params
			if tc.params.Limit == 0 {
				// Use defaultLimit if not specified, otherwise pagination logic handles it
			}
			if tc.params.Page == 0 {
				// Use 1 if not specified, otherwise pagination logic handles it
			}


			resultDocs, total, err := db.QueryDocuments(tc.params)

			if tc.expectErr {
				require.Error(t, err, "Expected an error but got none")
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains, "Error message mismatch")
				}
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
				assert.Equal(t, tc.expectedTotal, total, "Total matching count mismatch")

				// Extract IDs from result docs
				resultIDs := make([]string, len(resultDocs))
				for i, doc := range resultDocs {
					resultIDs[i] = doc.ID
				}
				assert.Equal(t, tc.expectedIDs, resultIDs, "Paginated document IDs mismatch")
			}
		})
	}
}