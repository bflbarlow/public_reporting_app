package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
	
	"reporting_app/internal/core"
	"reporting_app/internal/database"
	"reporting_app/internal/loader"
	"reporting_app/internal/security"
)

// RefreshHandler handles POST /refresh requests
type RefreshHandler struct {
	loader      *loader.Loader
	dbManager   *database.Manager
	nonceTracker *security.NonceTracker
	hmacSecret  []byte
	enablePublicPaths bool
	allowOrigins []string
}

// NewRefreshHandler creates a new refresh handler
func NewRefreshHandler(loader *loader.Loader, dbManager *database.Manager, nonceTracker *security.NonceTracker, hmacSecret []byte, enablePublicPaths bool, allowOrigins []string) *RefreshHandler {
	return &RefreshHandler{
		loader:      loader,
		dbManager:   dbManager,
		nonceTracker: nonceTracker,
		hmacSecret:  hmacSecret,
		enablePublicPaths: enablePublicPaths,
		allowOrigins: allowOrigins,
	}
}

// RefreshRequest represents the JSON request body for refresh
type RefreshRequest struct {
	Params map[string][]string `json:"params"`
}

// UnmarshalJSON custom unmarshaler to support both old (string) and new ([]string) formats
func (r *RefreshRequest) UnmarshalJSON(data []byte) error {
	// First, unmarshal into a raw map to handle any format
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}
	
	// Check if params field exists
	paramsRaw, ok := rawMap["params"]
	if !ok {
		// No params field is okay
		r.Params = make(map[string][]string)
		return nil
	}
	
	// Try to unmarshal params as map[string][]string first (new format)
	var paramsNew map[string][]string
	if err := json.Unmarshal(paramsRaw, &paramsNew); err == nil {
		r.Params = paramsNew
		return nil
	}
	
	// Try to unmarshal as map[string]string (old format)
	var paramsOld map[string]string
	if err := json.Unmarshal(paramsRaw, &paramsOld); err == nil {
		r.Params = make(map[string][]string)
		for k, v := range paramsOld {
			r.Params[k] = []string{v}
		}
		return nil
	}
	
	// Try to unmarshal as map[string]interface{} (mixed format)
	var paramsMixed map[string]interface{}
	if err := json.Unmarshal(paramsRaw, &paramsMixed); err == nil {
		r.Params = make(map[string][]string)
		for k, v := range paramsMixed {
			switch val := v.(type) {
			case string:
				r.Params[k] = []string{val}
			case []interface{}:
				result := make([]string, len(val))
				for i, item := range val {
					result[i] = fmt.Sprintf("%v", item)
				}
				r.Params[k] = result
			case nil:
				r.Params[k] = []string{""}
			default:
				// Try to convert anything else to string
				r.Params[k] = []string{fmt.Sprintf("%v", val)}
			}
		}
		return nil
	}
	
	// If we get here, params field exists but can't be parsed
	return fmt.Errorf("invalid params format")
}

// RefreshResponse represents the JSON response for refresh
type RefreshResponse struct {
	Data    map[string]interface{} `json:"data"`
	NextURL string                 `json:"next_url"`
	Error   string                 `json:"error,omitempty"`
}

// ServeHTTP handles refresh requests
func (h *RefreshHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS preflight requests
	if r.Method == http.MethodOptions {
		h.addCORSHeaders(w, r)
		w.WriteHeader(http.StatusOK)
		return
	}
	
	// Only POST requests
	if r.Method != http.MethodPost {
		h.addCORSHeaders(w, r)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Add CORS headers for actual POST request
	h.addCORSHeaders(w, r)
	
	// If public paths enabled, skip security validation
	if h.enablePublicPaths {
		query := r.URL.Query()
		reportID := query.Get("report_id")
		if reportID == "" {
			h.respondError(w, http.StatusBadRequest, "missing report_id")
			return
		}
		
		// Extract original parameters from URL
		originalParams := security.ExtractParams(query)
		
		// Load report
		report, err := h.loader.GetReport(reportID)
		if err != nil {
			h.respondError(w, http.StatusNotFound, fmt.Sprintf("Report not found: %s", reportID))
			return
		}
		
		// Parse new parameters from request body
		var refreshReq RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&refreshReq); err != nil {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
			return
		}
		
		// Merge parameters (original + new)
		finalParams := h.mergeParams(report, originalParams, refreshReq.Params)
		if finalParams == nil {
			h.respondError(w, http.StatusForbidden, "Parameter validation failed")
			return
		}
		
		// Validate final parameters
		if err := report.ValidateParams(finalParams); err != nil {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("Parameter validation failed: %v", err))
			return
		}
		
		// Execute datasource queries
		data, err := h.executeDatasources(report, finalParams)
		if err != nil {
			h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Query execution failed: %v", err))
			return
		}
		
		// Generate new URL for next refresh (without security)
		nextURL, err := h.generateNextURLPublic(report, finalParams)
		if err != nil {
			h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate next URL: %v", err))
			return
		}
		
		// Respond with data
		h.respondJSON(w, RefreshResponse{
			Data:    data,
			NextURL: nextURL,
		})
		return
	}
	
	// Parse original HMAC parameters from query string
	reportID, expires, nonce, sig, err := security.ParseSignedParams(r.URL.Query())
	if err != nil {
		h.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid parameters: %v", err))
		return
	}
	
	// Load report
	report, err := h.loader.GetReport(reportID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, fmt.Sprintf("Report not found: %s", reportID))
		return
	}
	
	// Extract original parameters from URL
	originalParams := security.ExtractParams(r.URL.Query())
	
	// Parse new parameters from request body
	var refreshReq RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&refreshReq); err != nil {
		h.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}
	
	// Merge parameters (original + new)
	finalParams := h.mergeParams(report, originalParams, refreshReq.Params)
	if finalParams == nil {
		h.respondError(w, http.StatusForbidden, "Parameter validation failed")
		return
	}
	
	// Validate final parameters
	if err := report.ValidateParams(finalParams); err != nil {
		h.respondError(w, http.StatusBadRequest, fmt.Sprintf("Parameter validation failed: %v", err))
		return
	}
	
	// Extract immutable parameters for HMAC verification
	immutableParams := report.ExtractImmutable(originalParams)
	
	// Verify original HMAC signature
	if !security.VerifyURL(reportID, expires, nonce, immutableParams, sig, h.hmacSecret) {
		h.respondError(w, http.StatusForbidden, "Invalid signature")
		return
	}
	
	// Check expiration with grace period
	now := time.Now().Unix()
	if expires < now-core.RefreshGraceSeconds {
		h.respondError(w, http.StatusForbidden, "URL expired (beyond grace period)")
		return
	}
	
	// Execute datasource queries
	data, err := h.executeDatasources(report, finalParams)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Query execution failed: %v", err))
		return
	}
	
	// Generate new URL for next refresh
	nextURL, err := h.generateNextURL(report, finalParams)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate next URL: %v", err))
		return
	}
	
	// Respond with data
	h.respondJSON(w, RefreshResponse{
		Data:    data,
		NextURL: nextURL,
	})
}

// mergeParams merges original and new parameters with validation
func (h *RefreshHandler) mergeParams(report *core.Report, original, new map[string][]string) map[string][]string {
	result := make(map[string][]string)
	
	// Start with original parameters
	for k, v := range original {
		result[k] = v
	}
	
	// Apply new parameters with validation
	for k, newValues := range new {
		// Check if parameter is defined in report
		if !report.ContainsParam(k) {
			log.Printf("SECURITY: Unknown parameter %s in refresh request", k)
			return nil
		}
		
		// Check immutable parameter boundaries
		if report.IsImmutable(k) {
			oldValues, hasOld := original[k]
			if hasOld {
				// For immutable parameters, compare slices
				if !slicesEqual(oldValues, newValues) {
					log.Printf("SECURITY: Attempt to change immutable parameter %s", k)
					return nil
				}
			}
			// If immutable param not previously set, we can add it
		}
		
		// Update parameter
		result[k] = newValues
	}
	
	return result
}

// slicesEqual compares two string slices for equality
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// executeDatasources executes all datasource queries
func (h *RefreshHandler) executeDatasources(report *core.Report, params map[string][]string) (map[string]interface{}, error) {
	// Execute each datasource (resolving database per-datasource)
	result := make(map[string]interface{})
	for name, ds := range report.Datasources {
		// Resolve database: datasource-level override, then report-level fallback
		dbName := ds.Database
		if dbName == "" {
			dbName = report.Database
		}
		
		// Get database connection
		db, err := h.dbManager.GetClient(dbName)
		if err != nil {
			return nil, fmt.Errorf("datasource %s: database connection failed: %w", name, err)
		}
		
		// Get connection config for defaults
		connConfig, err := h.dbManager.ConnectionConfig(dbName)
		if err != nil {
			return nil, fmt.Errorf("datasource %s: failed to get connection config: %w", name, err)
		}
		
		// Parse timeout
		timeout := 30 * time.Second
		if connConfig.Timeout != "" {
			d, err := time.ParseDuration(connConfig.Timeout)
			if err != nil {
				log.Printf("Invalid timeout %q, using default 30s: %v", connConfig.Timeout, err)
			} else {
				timeout = d
			}
		}
		
		// Determine row limit
		rowLimit := connConfig.RowLimit
		if report.MaxRows > 0 && report.MaxRows < rowLimit {
			rowLimit = report.MaxRows
		}
		
		queryResult, err := h.dbManager.ExecuteDatasource(db, ds, params, report, rowLimit, timeout, report.ID, name, dbName)
		if err != nil {
			return nil, fmt.Errorf("datasource %s: %w", name, err)
		}
		
		result[name] = map[string]interface{}{
			"columns": queryResult.Columns,
			"rows":    queryResult.Rows,
		}
	}
	
	return result, nil
}

// generateNextURL generates a new signed URL for the next refresh
func (h *RefreshHandler) generateNextURL(report *core.Report, params map[string][]string) (string, error) {
	// Generate new nonce (simple implementation)
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	
	// New expiration (5 minutes from now)
	expires := time.Now().Unix() + core.MaxURLExpirySeconds
	
	// Extract immutable parameters
	immutableParams := report.ExtractImmutable(params)
	
	// Sign URL
	sig := security.SignURL(report.ID, expires, nonce, immutableParams, h.hmacSecret)
	
	// Build URL
	urlStr := fmt.Sprintf("/api/embed?report_id=%s&expires=%d&nonce=%s&sig=%s",
		report.ID, expires, nonce, sig)
	
	// Add all parameters (including empty values for optional mutable parameters)
	for key, values := range params {
		for _, value := range values {
			// Always include parameter, even if empty
			urlStr += fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(value))
		}
	}
	
	return urlStr, nil
}

// generateNextURLPublic generates a URL without security for public paths
func (h *RefreshHandler) generateNextURLPublic(report *core.Report, params map[string][]string) (string, error) {
	// Build URL with just report_id and parameters
	urlStr := fmt.Sprintf("/api/embed?report_id=%s", report.ID)
	
	// Add all parameters (including empty values for optional mutable parameters)
	for key, values := range params {
		for _, value := range values {
			// Always include parameter, even if empty
			urlStr += fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(value))
		}
	}
	
	return urlStr, nil
}

// respondJSON sends a JSON response
func (h *RefreshHandler) respondJSON(w http.ResponseWriter, resp RefreshResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// respondError sends an error response
func (h *RefreshHandler) respondError(w http.ResponseWriter, status int, message string) {
	h.addCORSHeaders(w, nil) // Pass nil for request since we might not have it in all contexts
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(RefreshResponse{
		Error: message,
	})
}

// addCORSHeaders adds CORS headers based on allowOrigins configuration
func (h *RefreshHandler) addCORSHeaders(w http.ResponseWriter, r *http.Request) {
	// If enablePublicPaths is true, always allow any origin for development
	if h.enablePublicPaths {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "false")
		return
	}
	
	// Determine which origin to allow
	allowOrigin := ""
	
	// If allowOrigins contains "*", allow any origin
	for _, allowed := range h.allowOrigins {
		if allowed == "*" {
			allowOrigin = "*"
			break
		}
	}
	
	// If no wildcard and we have a request, check the Origin header
	if allowOrigin == "" && r != nil {
		origin := r.Header.Get("Origin")
		if origin != "" {
			for _, allowed := range h.allowOrigins {
				if allowed == origin {
					allowOrigin = origin
					break
				}
			}
		}
	}
	
	// Set CORS headers
	if allowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "false")
	}
}