package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	"reporting_app/internal/core"
	"reporting_app/internal/database"
	"reporting_app/internal/loader"
	"reporting_app/internal/security"
)

// EmbedHandler handles GET /api/embed requests
type EmbedHandler struct {
	loader      *loader.Loader
	dbManager   *database.Manager
	nonceTracker *security.NonceTracker
	hmacSecret  []byte
	enablePublicPaths bool
	allowOrigins    []string
	allowedCDNs     []string
}

// NewEmbedHandler creates a new embed handler
func NewEmbedHandler(loader *loader.Loader, dbManager *database.Manager, nonceTracker *security.NonceTracker, hmacSecret []byte, enablePublicPaths bool, allowOrigins []string, allowedCDNs []string) *EmbedHandler {
	return &EmbedHandler{
		loader:      loader,
		dbManager:   dbManager,
		nonceTracker: nonceTracker,
		hmacSecret:  hmacSecret,
		enablePublicPaths: enablePublicPaths,
		allowOrigins:    allowOrigins,
		allowedCDNs:     allowedCDNs,
	}
}

// ServeHTTP renders a report for iframe embedding
func (h *EmbedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS preflight requests
	if r.Method == http.MethodOptions {
		h.addCORSHeaders(w, r)
		w.WriteHeader(http.StatusOK)
		return
	}
	
	// Only GET requests
	if r.Method != http.MethodGet {
		h.addCORSHeaders(w, r)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Add CORS headers for actual GET request
	h.addCORSHeaders(w, r)
	
	// If public paths enabled, skip security validation
	if h.enablePublicPaths {
		query := r.URL.Query()
		reportID := query.Get("report_id")
		if reportID == "" {
			h.addCORSHeaders(w, r)
			http.Error(w, "missing report_id", http.StatusBadRequest)
			return
		}
		
		// Extract all parameters
		allParams := security.ExtractParams(query)
		
		// Load report
		report, err := h.loader.GetReport(reportID)
		if err != nil {
			h.addCORSHeaders(w, r)
			http.Error(w, fmt.Sprintf("Report not found: %s", reportID), http.StatusNotFound)
			return
		}
		
		// Validate parameters against report definition
		if err := report.ValidateParams(allParams); err != nil {
			h.addCORSHeaders(w, r)
			http.Error(w, fmt.Sprintf("Parameter validation failed: %v", err), http.StatusBadRequest)
			return
		}
		
		// Generate CSP header
		cspHeader := security.GenerateCSPHeader(h.allowOrigins, h.allowedCDNs)
		w.Header().Set("Content-Security-Policy", cspHeader)
		
		// Render report
		h.renderReport(w, r, report, allParams)
		return
	}
	
	// Parse and validate HMAC parameters
	reportID, expires, nonce, sig, err := security.ParseSignedParams(r.URL.Query())
	if err != nil {
		h.addCORSHeaders(w, r)
		http.Error(w, fmt.Sprintf("Invalid parameters: %v", err), http.StatusBadRequest)
		return
	}
	
	// Check expiration
	now := time.Now().Unix()
	if expires < now {
		h.addCORSHeaders(w, r)
		http.Error(w, "URL expired", http.StatusForbidden)
		return
	}
	
	// Reject if expires too far in future
	if expires > now+core.MaxURLExpirySeconds {
		h.addCORSHeaders(w, r)
		http.Error(w, "URL expires too far in future", http.StatusBadRequest)
		return
	}
	
	// Check nonce (prevent replay)
	if !h.nonceTracker.CheckAndAdd(nonce) {
		h.addCORSHeaders(w, r)
		http.Error(w, "Nonce already used", http.StatusForbidden)
		return
	}
	
	// Load report
	report, err := h.loader.GetReport(reportID)
	if err != nil {
		h.addCORSHeaders(w, r)
		http.Error(w, fmt.Sprintf("Report not found: %s", reportID), http.StatusNotFound)
		return
	}
	
	// Extract all parameters from URL
	allParams := security.ExtractParams(r.URL.Query())
	
	// Validate parameters against report definition
	if err := report.ValidateParams(allParams); err != nil {
		h.addCORSHeaders(w, r)
		http.Error(w, fmt.Sprintf("Parameter validation failed: %v", err), http.StatusBadRequest)
		return
	}
	
	// Extract immutable parameters for HMAC verification
	immutableParams := report.ExtractImmutable(allParams)
	
	// Verify HMAC signature
	if !security.VerifyURL(reportID, expires, nonce, immutableParams, sig, h.hmacSecret) {
		h.addCORSHeaders(w, r)
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}
	
	// Generate CSP header
	cspHeader := security.GenerateCSPHeader(h.allowOrigins, h.allowedCDNs)
	w.Header().Set("Content-Security-Policy", cspHeader)
	
	// Render report
	h.renderReport(w, r, report, allParams)
}

// renderReport renders the HTML for a report
func (h *EmbedHandler) renderReport(w http.ResponseWriter, r *http.Request, report *core.Report, params map[string][]string) {
	// 1. Construct path to dashboard.html
	htmlPath := filepath.Join("reports", report.ID, "dashboard.html")
	
	// 2. Read HTML file
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		log.Printf("Report HTML not found: %s", htmlPath)
		http.Error(w, "Report HTML not found", http.StatusInternalServerError)
		return
	}
	
	// 3. Generate ReportConfig JSON
	config := generateReportConfig(report, params, r.URL.String())
	
	// 4. Inject configuration into HTML
	htmlWithConfig := injectReportConfig(string(content), config)
	
	// 5. Serve
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlWithConfig))
}

// generateReportConfig creates the ReportConfig JSON structure
func generateReportConfig(report *core.Report, params map[string][]string, currentURL string) string {
	// Convert datasources to JSON
	datasourcesJSON, _ := json.Marshal(report.Datasources)
	
	// Convert params to JSON
	paramsJSON, _ := json.Marshal(params)
	
	// Build config object
	config := map[string]interface{}{
		"reportId":       report.ID,
		"reportName":     report.Name,
		"params":         string(paramsJSON),
		"immutableParams": report.ImmutableParams,
		"mutableParams":   report.MutableParams,
		"datasources":    string(datasourcesJSON),
		"currentUrl":     currentURL,
	}
	
	// Convert to JSON string
	configJSON, _ := json.Marshal(config)
	return string(configJSON)
}

// injectReportConfig injects the ReportConfig script before </body> tag
func injectReportConfig(htmlContent string, configJSON string) string {
	// Create the script tag with configuration
	configScript := fmt.Sprintf("<script>window.ReportConfig = %s;</script>", configJSON)
	
	// Find the </body> tag and insert script before it
	bodyCloseIndex := strings.LastIndex(htmlContent, "</body>")
	if bodyCloseIndex == -1 {
		// If no </body> tag found, append to end
		return htmlContent + "\n" + configScript
	}
	
	// Insert before </body>
	return htmlContent[:bodyCloseIndex] + configScript + "\n" + htmlContent[bodyCloseIndex:]
}

// addCORSHeaders adds CORS headers based on allowOrigins configuration
func (h *EmbedHandler) addCORSHeaders(w http.ResponseWriter, r *http.Request) {
	// If enablePublicPaths is true, always allow any origin for development
	if h.enablePublicPaths {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "false")
	}
}