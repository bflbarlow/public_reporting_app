package core

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
	
	// Security limits
	MaxURLExpirySeconds   = 300    // 5 minutes
	RefreshGraceSeconds   = 300    // 5 minutes grace for refresh
	NonceCleanupInterval  = 60     // seconds
)