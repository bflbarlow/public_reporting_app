package core

import "time"

const (
	// Default values
	DefaultRowLimit    = 10000
	DefaultExpiry      = 300 // 5 minutes in seconds
	DefaultCacheTTL    = 0   // no caching by default
	
	// Visibility options
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
	
	// URL parameter names
	ParamReportID = "report_id"
	ParamExpires  = "expires"
	ParamNonce    = "nonce"
	ParamSig      = "sig"
	
	// HMAC algorithm
	HMACAlgorithm = "sha256"
	
	// HTTP endpoints
	EndpointEmbed   = "/api/embed"
	EndpointRefresh = "/refresh"
	EndpointHealth  = "/health"
	EndpointStatic  = "/static/"
)

// DefaultSecurityConfig returns a SecurityConfig with all Phase 1 defaults.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		Nonce: NonceConfig{
			Bytes:           32,
			Encoding:        "urlsafe-base64",
			MaxAge:          24 * time.Hour,
			CleanupInterval: 60 * time.Second,
			MaxUses:         1,
			UseWindow:       5 * time.Minute,
		},
		Expiry: URLExpiryConfig{
			Default: 5 * time.Minute,
			Min:     1 * time.Minute,
			Max:     24 * time.Hour,
		},
		RefreshGrace: 0,
	}
}