package core

import (
	"errors"
	"fmt"
	"time"
)

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
	MultiValueParams []string // Parameters that support multiple values
	
	// Data sources (unified concept - charts or datasources)
	Datasources map[string]Datasource
}

// Datasource defines a single data query
type Datasource struct {
	Database string // datasource-level DB override (empty = use report.Database)
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
	Immutable map[string][]string
	Mutable   map[string][]string
}

// ExtractImmutable extracts immutable parameters from a full map
func (r *Report) ExtractImmutable(params map[string][]string) map[string][]string {
	result := make(map[string][]string)
	for _, name := range r.ImmutableParams {
		if value, ok := params[name]; ok {
			result[name] = value
		}
	}
	return result
}

// ValidateParams validates parameters against report definition
func (r *Report) ValidateParams(params map[string][]string) error {
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

// IsMultiValue checks if a parameter supports multiple values
func (r *Report) IsMultiValue(name string) bool {
	for _, n := range r.MultiValueParams {
		if n == name {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Security configuration types
// ---------------------------------------------------------------------------

// NonceConfig holds configuration for nonce generation and tracking.
type NonceConfig struct {
	Bytes           int           // number of random bytes (16-64)
	Encoding        string        // "urlsafe-base64", "base64", "hex"
	MaxAge          time.Duration // maximum age before auto-rejection
	CleanupInterval time.Duration // how often to run cleanup goroutine
	MaxUses         int           // max times a nonce can be used (1 = single-use)
	UseWindow       time.Duration // sliding window for multi-use nonces
}

// Validate checks that NonceConfig values are valid.
func (nc *NonceConfig) Validate() error {
	if nc.Bytes < 16 || nc.Bytes > 64 {
		return fmt.Errorf("NONCE_BYTES must be between 16 and 64, got %d", nc.Bytes)
	}
	switch nc.Encoding {
	case "urlsafe-base64", "base64", "hex":
		// valid
	default:
		return fmt.Errorf("NONCE_ENCODING must be urlsafe-base64, base64, or hex, got %q", nc.Encoding)
	}
	if nc.MaxAge <= 0 {
		return errors.New("NONCE_MAX_AGE must be greater than 0")
	}
	if nc.CleanupInterval <= 0 {
		return errors.New("NONCE_CLEANUP_INTERVAL must be greater than 0")
	}
	if nc.MaxUses < 1 {
		return errors.New("NONCE_MAX_USES must be at least 1")
	}
	if nc.UseWindow <= 0 {
		return errors.New("NONCE_USE_WINDOW must be greater than 0")
	}
	return nil
}

// URLExpiryConfig holds configuration for URL expiration.
type URLExpiryConfig struct {
	Default time.Duration // default URL expiration
	Min     time.Duration // minimum allowed expiration
	Max     time.Duration // maximum allowed expiration
}

// Validate checks that URLExpiryConfig values are consistent.
func (ue *URLExpiryConfig) Validate() error {
	if ue.Min <= 0 {
		return errors.New("URL_EXPIRY_MIN must be greater than 0")
	}
	if ue.Max <= 0 {
		return errors.New("URL_EXPIRY_MAX must be greater than 0")
	}
	if ue.Min > ue.Max {
		return fmt.Errorf("URL_EXPIRY_MIN (%s) must not exceed URL_EXPIRY_MAX (%s)", ue.Min, ue.Max)
	}
	if ue.Default < ue.Min || ue.Default > ue.Max {
		return fmt.Errorf("URL_EXPIRY_DEFAULT (%s) must be between MIN (%s) and MAX (%s)", ue.Default, ue.Min, ue.Max)
	}
	return nil
}

// SecurityConfig aggregates all security-related settings.
type SecurityConfig struct {
	Nonce        NonceConfig
	Expiry       URLExpiryConfig
	RefreshGrace time.Duration
}

// Validate checks all nested configs.
func (sc *SecurityConfig) Validate() error {
	if err := sc.Nonce.Validate(); err != nil {
		return fmt.Errorf("nonce config: %w", err)
	}
	if err := sc.Expiry.Validate(); err != nil {
		return fmt.Errorf("expiry config: %w", err)
	}
	if sc.RefreshGrace < 0 {
		return errors.New("REFRESH_GRACE_PERIOD must be >= 0")
	}
	return nil
}