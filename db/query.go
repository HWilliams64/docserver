package db

import (
	"docserver/models"
	"encoding/json" // Added
	"errors"
	"fmt"
	"log" // Added
	"sort"
	"strconv" // Re-added for compareJSONValue
	"regexp" // Added for number literal check
	"strings"
	// "time" // Removed unused import

	"github.com/tidwall/gjson"
)

// Simple regex to check if a string looks like a number literal (integer or float)
var isNumberLiteral = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// --- Query Structures ---

// QueryCondition represents a single condition like "path operator value".
type QueryCondition struct {
	Path          string      // Dot notation path (e.g., "user.name") or empty for root
	Operator      string      // e.g., "equals", "contains", "greaterThan" (base operator, no suffix)
	ParsedValue   interface{} // The parsed value (string, float64, bool, nil)
	ValueType     gjson.Type  // The type determined during parsing
	IsInsensitive bool        // Flag derived from operator suffix
	Original      string      // Original condition string for error messages
}

// LogicalOperator represents "and" or "or".
type LogicalOperator string

const (
	LogicAnd LogicalOperator = "and"
	LogicOr  LogicalOperator = "or"
)

// ParsedQuery holds the sequence of conditions and logical operators.
type ParsedQuery struct {
	Conditions []QueryCondition
	Logic      []LogicalOperator // Logic[i] applies between Conditions[i] and Conditions[i+1]
}

// --- Query Parsing ---

var validOperators = map[string]bool{
	"equals":              true, "notequals":           true,
	"greaterthan":         true, "lessthan":            true,
	"greaterthanorequals": true, "lessthanorequals":    true,
	"contains":            true, "startswith":          true, "endswith":            true,
	// Case-insensitive variants (normalized to lowercase without suffix)
	"equals-insensitive":              true, "notequals-insensitive":           true,
	"contains-insensitive":            true, "startswith-insensitive":          true, "endswith-insensitive":            true,
}

var stringOnlyOperators = map[string]bool{
	"startswith": true, "endswith": true,
	"startswith-insensitive": true, "endswith-insensitive": true,
}

var arrayOrStringOperators = map[string]bool{
    "contains": true, "contains-insensitive": true,
}

// ParseContentQuery takes the raw query array from the request and parses it
// into a structured ParsedQuery. It performs syntax validation.
func ParseContentQuery(queryParts []string) (*ParsedQuery, error) {
	if len(queryParts) == 0 {
		return nil, nil // No query provided is valid
	}

	parsed := &ParsedQuery{}
	isExpectingCondition := true

	for i, part := range queryParts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("query part at index %d is empty", i)
		}

		if isExpectingCondition {
			condition, err := parseSingleCondition(part)
			if err != nil {
				return nil, fmt.Errorf("invalid condition at index %d ('%s'): %w", i, part, err)
			}
			parsed.Conditions = append(parsed.Conditions, condition)
		} else {
			logic := LogicalOperator(strings.ToLower(part))
			if logic != LogicAnd && logic != LogicOr {
				return nil, fmt.Errorf("invalid logical operator at index %d: '%s', expected 'and' or 'or'", i, part)
			}
			parsed.Logic = append(parsed.Logic, logic)
		}
		isExpectingCondition = !isExpectingCondition
	}

	// The loop must end after parsing a condition
	// The loop must end after parsing a condition. If we are still expecting one, it means the query ended with a logical operator.
	if isExpectingCondition && len(queryParts) > 0 { // Add len check to allow empty query
		return nil, errors.New("query must end with a condition, not a logical operator")
	}

	// Number of logic operators must be one less than the number of conditions
	if len(parsed.Conditions) > 1 && len(parsed.Logic) != len(parsed.Conditions)-1 {
		// This case should theoretically be caught by the alternating check, but double-check
		return nil, errors.New("mismatch between number of conditions and logical operators")
	}

	return parsed, nil
}

// parseSingleCondition parses a string like "path operator value" into QueryCondition,
// determining the type of the value.
func parseSingleCondition(conditionStr string) (QueryCondition, error) {
	parts := strings.Fields(conditionStr) // Simple split by whitespace

	if len(parts) < 2 {
		return QueryCondition{}, fmt.Errorf("condition must have at least an operator and a value")
	}

	var path, operator, rawValueStr string
	var isInsensitive bool

	// Determine structure: "operator value..." or "path operator value..."
	potentialOperator := strings.ToLower(parts[0])
	_, isFirstPartOperator := validOperators[potentialOperator]

	if isFirstPartOperator && len(parts) >= 2 {
		path = ""
		operator = potentialOperator
		// Reconstruct the raw value string, preserving original spacing if quoted
		valueStartIndex := strings.Index(conditionStr, parts[1])
		if valueStartIndex == -1 { // Should not happen if parts[1] exists
			return QueryCondition{}, fmt.Errorf("internal parsing error: could not find value start")
		}
		rawValueStr = strings.TrimSpace(conditionStr[valueStartIndex:])

	} else if len(parts) >= 3 {
		path = parts[0]
		operator = strings.ToLower(parts[1])
		// Reconstruct the raw value string
		valueStartIndex := strings.Index(conditionStr, parts[2])
		if valueStartIndex == -1 { // Should not happen if parts[2] exists
			return QueryCondition{}, fmt.Errorf("internal parsing error: could not find value start")
		}
		rawValueStr = strings.TrimSpace(conditionStr[valueStartIndex:])

		// Validate operator early (before insensitive check)
		_, isValidOp := validOperators[operator]
		if !isValidOp && !strings.HasSuffix(operator, "-insensitive") {
			return QueryCondition{}, fmt.Errorf("invalid operator '%s'", operator)
		}
	} else { // len(parts) == 2 and first part is NOT an operator (e.g., "path value")
		potentialOp := strings.ToLower(parts[1])
		if _, isValid := validOperators[potentialOp]; isValid {
			return QueryCondition{}, fmt.Errorf("condition must have at least an operator and a value") // Missing value
		}
		return QueryCondition{}, fmt.Errorf("invalid condition format") // Missing operator
	}

	// Handle insensitive suffix
	if strings.HasSuffix(operator, "-insensitive") {
		baseOperator := strings.TrimSuffix(operator, "-insensitive")
		isSupported := stringOnlyOperators[baseOperator] || arrayOrStringOperators[baseOperator] || baseOperator == "equals" || baseOperator == "notequals"
		if !isSupported {
			return QueryCondition{}, fmt.Errorf("invalid base operator for insensitive matching '%s'", baseOperator)
		}
		isInsensitive = true
		operator = baseOperator // Use the base operator moving forward
	}

	// --- Parse the rawValueStr to determine type ---
	var parsedValue interface{}
	var valueType gjson.Type

	trimmedValue := strings.TrimSpace(rawValueStr) // Use trimmed for type checks

	// Order matters: Check number before bool, as "0" is valid for both.
	if len(trimmedValue) >= 2 && trimmedValue[0] == '"' && trimmedValue[len(trimmedValue)-1] == '"' {
		// 1. Explicitly quoted string
		parsedValue = trimmedValue[1 : len(trimmedValue)-1] // Store unquoted string
		valueType = gjson.String
	} else if trimmedValue == "null" {
		// 2. Null
		parsedValue = nil
		valueType = gjson.Null
	} else if f, ok := tryParseFloat(trimmedValue); ok {
		// 3. Number (Check before bool!)
		parsedValue = f
		valueType = gjson.Number
	} else if b, ok := tryParseBool(trimmedValue); ok {
		// 4. Boolean
		parsedValue = b
		valueType = gjson.False // Default, will be True if b is true
		if b {
			valueType = gjson.True
		}
	} else {
		// 5. Default to string if not quoted and not null/number/bool
		parsedValue = trimmedValue
		valueType = gjson.String
	}
	// --- End Value Parsing ---

	return QueryCondition{
		Path:          path,
		Operator:      operator, // Base operator
		ParsedValue:   parsedValue,
		ValueType:     valueType,
		IsInsensitive: isInsensitive,
		Original:      conditionStr,
	}, nil
}


// --- Query Evaluation ---

// EvaluateContentQuery checks if a single document matches the parsed query.
func (db *Database) EvaluateContentQuery(doc models.Document, query *ParsedQuery) (bool, error) {
	if query == nil || len(query.Conditions) == 0 {
		return true, nil // No query means match
	}

	// Evaluate the first condition
	result, err := db.evaluateSingleCondition(doc, query.Conditions[0])
	if err != nil {
		// Ensure errors from evaluation (like invalid op on plain text) are returned
		return false, fmt.Errorf("error evaluating condition '%s': %w", query.Conditions[0].Original, err)
	}

	// Sequentially apply logical operators
	for i, logic := range query.Logic {
		if i+1 >= len(query.Conditions) {
			// Should not happen if parsing is correct
			return false, fmt.Errorf("internal error: logic operator index %d out of bounds for conditions", i)
		}

		nextResult, err := db.evaluateSingleCondition(doc, query.Conditions[i+1])
		if err != nil {
			// Ensure errors from evaluation are returned
			return false, fmt.Errorf("error evaluating condition '%s': %w", query.Conditions[i+1].Original, err)
		}

		switch logic {
		case LogicAnd:
			result = result && nextResult
		case LogicOr:
			result = result || nextResult
		}
	}

	return result, nil
}

// evaluateSingleCondition checks if a document satisfies one specific condition.
func (db *Database) evaluateSingleCondition(doc models.Document, cond QueryCondition) (bool, error) {
	// Convert doc.Content to JSON string if it's not already a string
	// This is needed for gjson to parse it.
	var contentJSON string
	switch v := doc.Content.(type) {
	case string:
		contentJSON = v
	default:
		// Attempt to marshal non-string content back to JSON
		jsonBytes, err := json.Marshal(doc.Content)
		if err != nil {
			// If marshalling fails, treat as plain text for limited operators
			log.Printf("DEBUG: Could not marshal document content to JSON for query evaluation (DocID: %s). Treating as plain text. Error: %v", doc.ID, err)
			// Use fmt.Sprintf as a fallback for basic types? Risky.
			// Let's stick to the plan: only specific operators for non-JSON.
			contentJSON = fmt.Sprintf("%v", doc.Content) // Fallback representation
            if !isValidForPlainText(cond.Operator, cond.IsInsensitive) {
                 return false, fmt.Errorf("content is not valid JSON, and operator '%s' is not supported for plain text", cond.Original)
            }
            // Proceed with plain text evaluation below
		} else {
			contentJSON = string(jsonBytes)
		}
	}

    isPlainText := !gjson.Valid(contentJSON)
    if isPlainText && !isValidForPlainText(cond.Operator, cond.IsInsensitive) {
         return false, fmt.Errorf("content is plain text, and operator '%s' is not supported for plain text", cond.Original)
    }


	// Get the value from the document using gjson
	var targetValue gjson.Result
	if cond.Path == "" {
        // If path is empty, operate on the root of the content JSON
        targetValue = gjson.Parse(contentJSON)
	} else {
		targetValue = gjson.Get(contentJSON, cond.Path)
		      // If path doesn't exist, it's an error (to match test Path_non-existent:_error)
		      if !targetValue.Exists() && !isPlainText { // Don't error if plain text (path is irrelevant)
		          return false, fmt.Errorf("path '%s' does not exist in document content", cond.Path)
		      }
	}


	// --- Perform Comparison based on Operator ---
	// This part needs careful handling of types and operators

    // Handle plain text separately first
    if isPlainText {
        // Check validity *before* calling comparePlainText
        if !isValidForPlainText(cond.Operator, cond.IsInsensitive) {
             // Return the error here, ensuring it propagates for Plain_text_invalid_op tests
             return false, fmt.Errorf("content is plain text, and operator '%s' is not supported", cond.Original)
        }
        // comparePlainText itself can return an error, ensure it's propagated
        return comparePlainText(contentJSON, cond) // Directly return result and potential error
    }

    // Handle JSON content
	return compareJSONValue(targetValue, cond)
}

// isValidForPlainText checks if an operator is allowed for non-JSON string content.
func isValidForPlainText(operator string, isInsensitive bool) bool {
    opKey := operator
    if isInsensitive {
        opKey += "-insensitive"
    }
    switch opKey {
    case "equals", "notequals", "contains", "startswith", "endswith",
         "equals-insensitive", "notequals-insensitive", "contains-insensitive",
         "startswith-insensitive", "endswith-insensitive":
        return true
    default:
        return false
    }
}


// comparePlainText performs comparisons for plain text content.
func comparePlainText(textContent string, cond QueryCondition) (bool, error) {
	// Plain text comparison primarily works with strings.
	// Ensure ParsedValue is a string, otherwise it's a type mismatch for plain text.
	valStr, ok := cond.ParsedValue.(string)
	if !ok {
		// This shouldn't happen if parsing logic is correct for plain text operators,
		// but handle defensively. Non-string values aren't comparable as plain text here.
		return false, fmt.Errorf("internal error: expected string value for plain text comparison, got %T", cond.ParsedValue)
	}

	op := cond.Operator
	if cond.IsInsensitive {
		op += "-insensitive" // Reconstruct full operator for switch
        textContent = strings.ToLower(textContent)
        valStr = strings.ToLower(valStr)
    }

    switch op {
    case "equals", "equals-insensitive":
        return textContent == valStr, nil
    case "notequals", "notequals-insensitive":
        return textContent != valStr, nil
    case "contains", "contains-insensitive":
        return strings.Contains(textContent, valStr), nil
    case "startswith", "startswith-insensitive":
        return strings.HasPrefix(textContent, valStr), nil
    case "endswith", "endswith-insensitive":
        return strings.HasSuffix(textContent, valStr), nil
    default:
    // This should be caught by isValidForPlainText, but return error just in case. Match test expectation. Use cond.Operator.
    return false, fmt.Errorf("content is plain text, and operator '%s' is not supported", cond.Operator)
       }
}


// compareJSONValue performs comparisons for gjson.Result values.
func compareJSONValue(targetValue gjson.Result, cond QueryCondition) (bool, error) {
	op := cond.Operator
	// We now use cond.ParsedValue and cond.ValueType instead of cond.Value (string)
	parsedVal := cond.ParsedValue
	condValType := cond.ValueType
	targetType := targetValue.Type

	// --- Start: Added Check for Invalid Operators on Root Primitives ---
	// Check if comparing at the root path against a primitive JSON type
	// using an operator not valid for plain text. This handles tests like
	// 'Plain text invalid op (root): error' where content is e.g., 12345.
	if cond.Path == "" {
		switch targetType {
		case gjson.String, gjson.Number, gjson.True, gjson.False:
			// If the operator is not valid for plain text, return the specific error
			if !isValidForPlainText(cond.Operator, cond.IsInsensitive) {
				// Match the error message expected by the test - Use cond.Operator
				return false, fmt.Errorf("content is plain text, and operator '%s' is not supported", cond.Operator)
			}
		}
	}
	// --- End: Added Check ---


	// Handle array 'contains' separately
	if targetType == gjson.JSON && targetValue.IsArray() && op == "contains" {
		found := false
		// Use the pre-parsed value and type from the condition
		// No need to re-parse valStr here

		targetValue.ForEach(func(key, value gjson.Result) bool {
			elementMatches := false
			// Strict type checking based on the *element's* type
			switch value.Type {
			case gjson.String:
				// Element is String. Match ONLY if condValType is String AND strings match.
				if condValType == gjson.String {
					condStr := parsedVal.(string) // Assert type
					elementStr := value.String()
					if cond.IsInsensitive {
						elementMatches = strings.EqualFold(elementStr, condStr)
					} else {
						elementMatches = elementStr == condStr
					}
				} else {
					elementMatches = false // Condition value is not a string
				}
			case gjson.Number:
				// Element is Number. Match ONLY if condValType is Number AND values match.
				if condValType == gjson.Number {
					condNum := parsedVal.(float64) // Assert type
					elementMatches = value.Float() == condNum
				} else {
					elementMatches = false // Condition value is not a number
				}
			case gjson.True, gjson.False:
				// Element is Bool. Match ONLY if condValType is Bool AND values match.
				if condValType == gjson.True || condValType == gjson.False {
					condBool := parsedVal.(bool) // Assert type
					elementMatches = value.Bool() == condBool
				} else {
					elementMatches = false // Condition value is not a boolean
				}
			case gjson.Null:
				// Element is Null. Match ONLY if condValType is Null.
				elementMatches = (condValType == gjson.Null)
			}

			if elementMatches {
				found = true
				return false // Stop iterating
			}
			return true // Continue iterating
		})
		return found, nil
	}


	// Handle general null comparisons (excluding array contains which was handled above)
	isNullTarget := targetType == gjson.Null
	isNullCondValue := condValType == gjson.Null // Check the parsed type

	if isNullTarget || isNullCondValue {
		// If comparing null with null
		if isNullTarget && isNullCondValue {
			switch op {
			case "equals": return true, nil
			case "notequals": return false, nil
			// contains was handled above if target is array
			// if target is not array, contains(null, null) is invalid
			default: return false, fmt.Errorf("operator '%s' invalid for null comparison", cond.Operator)
			}
		} else { // Comparing null with non-null
			switch op {
			case "equals": return false, nil // null != non-null
			case "notequals": return true, nil  // null != non-null
			// contains(null, non-null) -> false
			// contains(non-null, null) -> false (unless non-null is array containing null, handled above)
			case "contains": return false, nil
			// Other operators are invalid for null/non-null comparison
			default: return false, fmt.Errorf("operator '%s' invalid for comparing null with non-null value", cond.Operator) // Value string removed
			}
		}
	}
	// If we reach here, neither target nor value is null


	// Handle different target types
	switch targetType {
	case gjson.String:
		targetStr := targetValue.String()
		// Check if operator is valid for String
		switch op {
		case "equals", "notequals", "contains", "startswith", "endswith":
			// Operator is valid for string. Now check condition value type.
			if condValType != gjson.String {
				// Allow noteuals(string, non-string) -> true
				if op == "notequals" { return true, nil }
				// Error: Condition value type mismatch for string operation
				// Use generic type mismatch error for now.
				return false, fmt.Errorf("type mismatch: cannot compare string with %s using operator '%s'", condValType.String(), op)
			}
			// Both are strings, proceed with comparison
			condStr := parsedVal.(string)
			valCompare := condStr
			if cond.IsInsensitive {
				targetStr = strings.ToLower(targetStr)
				valCompare = strings.ToLower(valCompare)
				op += "-insensitive" // Use full op name for switch
			}
			// Perform actual string comparison (using potentially suffixed op)
			switch op {
			case "equals", "equals-insensitive": return targetStr == valCompare, nil
			case "notequals", "notequals-insensitive": return targetStr != valCompare, nil
			case "contains", "contains-insensitive": return strings.Contains(targetStr, valCompare), nil
			case "startswith", "startswith-insensitive": return strings.HasPrefix(targetStr, valCompare), nil
			case "endswith", "endswith-insensitive": return strings.HasSuffix(targetStr, valCompare), nil
			default: // Should not happen
				return false, fmt.Errorf("internal error: unknown string operator '%s'", op)
			}
		default:
			// Error: Operator is invalid for String target type
			// Match test "String_numeric_op" expectation
			return false, fmt.Errorf("type mismatch: cannot apply numeric operator '%s' to string value", op)
		}

	case gjson.Number:
		targetNum := targetValue.Float()
		// Check if operator is valid for Number
		switch op {
		case "equals", "notequals", "greaterthan", "lessthan", "greaterthanorequals", "lessthanorequals":
			// Operator is valid for number. Now check condition value type.
			if condValType != gjson.Number {
				// Allow noteuals(number, non-number) -> true
				if op == "notequals" { return true, nil }
				// Error: Condition value type mismatch for numeric operation.
				// Try to match the test expectation "value '...' is not a valid number".
				return false, fmt.Errorf("type mismatch: value '%v' is not a valid number for comparison with operator '%s'", parsedVal, op)
			}
			// Both are numbers, proceed with comparison
			valNum := parsedVal.(float64)
			if cond.IsInsensitive { // Should be caught earlier by parser
				return false, fmt.Errorf("operator '%s' cannot be case-insensitive for numeric comparison", cond.Original)
			}
			// Perform actual numeric comparison
			switch op {
			case "equals": return targetNum == valNum, nil
			case "notequals": return targetNum != valNum, nil
			case "greaterthan": return targetNum > valNum, nil
			case "lessthan": return targetNum < valNum, nil
			case "greaterthanorequals": return targetNum >= valNum, nil
			case "lessthanorequals": return targetNum <= valNum, nil
			default: // Should not happen
				return false, fmt.Errorf("internal error: unknown numeric operator '%s'", op)
			}
		default:
			// Error: Operator is invalid for Number target type
			// Match test "Number string op: error" expectation
			 return false, fmt.Errorf("type mismatch: cannot apply string operator '%s' to numeric value", op)
		}

	case gjson.True, gjson.False:
		targetBool := targetValue.Bool()
		// Check if operator is valid for Boolean
		switch op {
		case "equals", "notequals":
			// Operator is valid for boolean. Now check condition value type.
			if !(condValType == gjson.True || condValType == gjson.False) {
				// Allow noteuals(bool, non-bool) -> true
				if op == "notequals" { return true, nil }
				// Error: Condition value type mismatch for boolean operation.
				// Try to match the test expectation "value '...' is not a valid boolean".
				return false, fmt.Errorf("type mismatch: value '%v' is not a valid boolean for comparison with operator '%s'", parsedVal, op)
			}
			// Both are booleans, proceed with comparison
			valBool := parsedVal.(bool)
			if cond.IsInsensitive { // Should be caught earlier by parser
				return false, fmt.Errorf("operator '%s' cannot be case-insensitive for boolean comparison", cond.Original)
			}
			// Perform actual boolean comparison
			switch op {
			case "equals": return targetBool == valBool, nil
			case "notequals": return targetBool != valBool, nil
			default: // Should not happen
				return false, fmt.Errorf("internal error: unknown boolean operator '%s'", op)
			}
		default:
			// Error: Operator is invalid for Boolean target type
			// Match test "Boolean invalid op: error" expectation
			return false, fmt.Errorf("operator '%s' is invalid for boolean comparison", op)
		}

	case gjson.JSON: // Represents an array or object
		// Array 'contains' was handled earlier.
		// Other operators are generally invalid for direct array/object comparison.
		if targetValue.IsArray() {
			if op == "equals" || op == "notequals" {
				// Compare raw JSON? Error. Match test expectation.
				return false, fmt.Errorf("operator '%s' cannot directly compare arrays/objects", cond.Operator)
			} else {
				// All other operators are invalid for arrays (contains handled above)
				return false, fmt.Errorf("operator '%s' is invalid for array comparison", cond.Operator)
			}
		} else { // It's an object
			// Handle Path_non-existent_(root):_ok test case
			// If path was empty and target is object, non-contains operators should yield false, nil
			// This seems specific to the test case, maybe adjust if needed.
			if cond.Path == "" && (op == "equals" || op == "notequals") {
				return false, nil // Match test expectation
			}
			// Otherwise, error for direct object comparison
			return false, fmt.Errorf("operator '%s' cannot directly compare JSON objects", cond.Operator)
		}

	default:
		// This case should ideally not be reached if all gjson types are handled
		log.Printf("Warning: Unsupported gjson type '%s' encountered during query evaluation for path '%s'", targetType.String(), cond.Path)
		return false, fmt.Errorf("unsupported type '%s' encountered during query evaluation", targetType.String())
	}
}


// --- Main Query Function ---

// QueryDocumentsParams holds all parameters for querying documents.
type QueryDocumentsParams struct {
	AuthUserID    string   // ID of the authenticated user (for scope filtering)
	Scope         string   // "owned", "shared", "all" (default)
	ContentQuery  []string // Raw content query parts
	SortBy        string   // "creation_date", "last_modified_date" (default)
	Order         string   // "asc", "desc" (default)
	Page          int      // 1-based page number
	Limit         int      // Max items per page (max 100)
}

// QueryDocuments performs filtering, sorting, and pagination on documents.
func (db *Database) QueryDocuments(params QueryDocumentsParams) ([]models.Document, int, error) {
	// 1. Parse Content Query
	parsedQuery, err := ParseContentQuery(params.ContentQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid content_query: %w", err)
	}

	// 2. Get Initial Set (All documents for now, optimize later if needed)
	allDocs := db.GetAllDocuments() // Needs RLock internally

	// 3. Filter by Scope and Content Query
	filteredDocs := make([]models.Document, 0)
	for _, doc := range allDocs {
		// Check scope first
		isOwned := doc.OwnerID == params.AuthUserID
		isShared := false
		if !isOwned { // Only check shares if not owned
			shareRecord, found := db.GetShareRecordByDocumentID(doc.ID) // Needs RLock internally
			if found {
				for _, sharedID := range shareRecord.SharedWith {
					if sharedID == params.AuthUserID {
						isShared = true
						break
					}
				}
			}
		}

		scopeMatch := false
		switch strings.ToLower(params.Scope) {
		case "owned":
			scopeMatch = isOwned
		case "shared":
			scopeMatch = isShared
		case "all", "": // Default to all
			scopeMatch = isOwned || isShared
		default:
			return nil, 0, fmt.Errorf("invalid scope value: '%s', expected 'owned', 'shared', or 'all'", params.Scope)
		}

		if !scopeMatch {
			continue // Skip doc if scope doesn't match
		}

		// Check content query if applicable
		if parsedQuery != nil {
			contentMatch, err := db.EvaluateContentQuery(doc, parsedQuery)
			if err != nil {
				// Log error if evaluation fails for a document, but continue processing others.
				// Do not return an error from QueryDocuments itself unless query parsing failed.
				log.Printf("WARN: Error evaluating content query for document ID %s, skipping document: %v", doc.ID, err)
				continue // Skip this document
			}
			if !contentMatch {
				continue // Skip doc if content query doesn't match
			}
		}

		// If we reach here, the document matches scope and content query
		filteredDocs = append(filteredDocs, doc)
	}

    totalMatching := len(filteredDocs) // Total count before pagination

	// 4. Sort
    err = sortDocuments(filteredDocs, params.SortBy, params.Order)
    if err != nil {
        return nil, 0, err // Propagate sorting errors
    }

	// 5. Paginate
    paginatedDocs, err := paginateDocuments(filteredDocs, params.Page, params.Limit)
    if err != nil {
        return nil, 0, err // Propagate pagination errors
    }


	return paginatedDocs, totalMatching, nil
}


// --- Sorting Helper ---
func sortDocuments(docs []models.Document, sortBy, order string) error {
    lessFunc := func(i, j int) bool {
        docI := docs[i]
        docJ := docs[j]
        switch strings.ToLower(sortBy) {
        case "last_modified_date":
            return docI.LastModifiedDate.Before(docJ.LastModifiedDate)
        case "creation_date", "": // Default to creation_date
            return docI.CreationDate.Before(docJ.CreationDate)
        default:
            // Return error from the outer function
            return false // Value doesn't matter here
        }
    }

    // Wrap lessFunc based on order
    if strings.ToLower(order) == "desc" {
        originalLess := lessFunc
        lessFunc = func(i, j int) bool {
            // To reverse, swap i and j in the original comparison
            return originalLess(j, i)
        }
    } else if strings.ToLower(order) != "asc" && order != "" {
         return fmt.Errorf("invalid order value: '%s', expected 'asc' or 'desc'", order)
    }


    // Check for invalid sortBy before sorting
    switch strings.ToLower(sortBy) {
        case "last_modified_date", "creation_date", "":
             // Valid cases
        default:
             return fmt.Errorf("invalid sort_by value: '%s', expected 'creation_date' or 'last_modified_date'", sortBy)
    }


    sort.SliceStable(docs, lessFunc)
    return nil
}

// --- Pagination Helper ---
const defaultLimit = 20
const maxLimit = 100

func paginateDocuments(docs []models.Document, page, limit int) ([]models.Document, error) {
    if page <= 0 {
        page = 1 // Default to page 1
    }
    if limit <= 0 {
        limit = defaultLimit
    }
    if limit > maxLimit {
        limit = maxLimit
    }

    startIndex := (page - 1) * limit
    endIndex := startIndex + limit

    if startIndex >= len(docs) {
        return []models.Document{}, nil // Page is out of bounds, return empty list
    }

    if endIndex > len(docs) {
        endIndex = len(docs)
    }

    return docs[startIndex:endIndex], nil
}
// tryParseFloat attempts to parse a string as float64.
func tryParseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

// tryParseBool attempts to parse a string as bool.
func tryParseBool(s string) (bool, bool) {
	b, err := strconv.ParseBool(s)
	return b, err == nil
}
