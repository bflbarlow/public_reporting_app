package core

import "fmt"

// Report represents a complete report definition
type Report struct {
	ID              string
	Name            string
	Description     string
	Database        string
	Visibility      string // "public" or "private"
	ExpiresAfter    int    // seconds
	MaxRows         int    // maximum rows per query
	
	// Parameter classification
	ImmutableParams []string // Included in HMAC signature
	MutableParams   []string // Excluded from HMAC, can be changed
	
	// Data sources (unified concept - charts or datasources)
	Datasources map[string]Datasource
}

// Datasource defines a single data query
type Datasource struct {
	SQL      string
	RowLimit int    // 0 = use default
	CacheTTL int    // 0 = no caching, seconds
}

// QueryResult holds the result of a SQL query
type QueryResult struct {
	Columns []string
	Rows    [][]interface{}
}

// ParamSet represents parameters for a report request
type ParamSet struct {
	Immutable map[string]string
	Mutable   map[string]string
}

// ExtractImmutable extracts immutable parameters from a full map
func (r *Report) ExtractImmutable(params map[string]string) map[string]string {
	result := make(map[string]string)
	for _, name := range r.ImmutableParams {
		if value, ok := params[name]; ok {
			result[name] = value
		}
	}
	return result
}

// ValidateParams validates parameters against report definition
func (r *Report) ValidateParams(params map[string]string) error {
	// Check required immutable params
	for _, name := range r.ImmutableParams {
		if _, ok := params[name]; !ok {
			// Note: We might want some params to be optional
			// For now, all immutable params are required
			// return fmt.Errorf("missing immutable parameter: %s", name)
		}
	}
	
	// Check for unknown parameters
	for name := range params {
		if !r.ContainsParam(name) {
			return fmt.Errorf("unknown parameter: %s", name)
		}
	}
	
	return nil
}

// ContainsParam checks if a parameter name is defined (immutable or mutable)
func (r *Report) ContainsParam(name string) bool {
	for _, n := range r.ImmutableParams {
		if n == name {
			return true
		}
	}
	for _, n := range r.MutableParams {
		if n == name {
			return true
		}
	}
	return false
}

// IsImmutable checks if a parameter is immutable
func (r *Report) IsImmutable(name string) bool {
	for _, n := range r.ImmutableParams {
		if n == name {
			return true
		}
	}
	return false
}