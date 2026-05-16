package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"reporting_app/internal/core"
	"reporting_app/internal/database"
	"reporting_app/internal/handler"
	"reporting_app/internal/loader"
	"reporting_app/internal/logging"
	"reporting_app/internal/security"
	"reporting_app/internal/server"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found, using environment variables")
	}

	// Parse flags
	genURL := flag.Bool("genurl", false, "Generate a signed URL")
	reportID := flag.String("report", "", "Report ID for URL generation")
	expiresSec := flag.Int("expires", 300, "Expiration in seconds from now")
	paramsStr := flag.String("params", "", "Parameters as key=value|key=value (use | to separate pairs, , for multi-value)")
	reloadSnippets := flag.Bool("reload-snippets", false, "Reload snippets from disk and print count")
	flag.Parse()

	if *reloadSnippets {
		// Load snippets and print count
		snippetsDir := os.Getenv("SNIPPETS_DIR")
		if snippetsDir == "" {
			snippetsDir = "./snippets"
		}
		snippets, err := loader.LoadSnippets(snippetsDir)
		if err != nil {
			log.Fatalf("Failed to load snippets from %s: %v", snippetsDir, err)
		}
		log.Printf("📦 Loaded %d snippets from %s", len(snippets), snippetsDir)
		for name, snippet := range snippets {
			log.Printf("  - %s: %s", name, snippet.Description)
		}
		return
	}

	if *genURL {
		// Load config early for URL generation
		cfg := loadConfig()
		generateURL(*reportID, *expiresSec, *paramsStr, cfg.securityConfig)
		return
	}

	// Configuration
	config := loadConfig()

	// Initialize components
	reportLoader, err := loader.New(config.reportsDir)
	if err != nil {
		log.Fatalf("Failed to load reports: %v", err)
	}
	log.Printf("Loaded %d reports", len(reportLoader.ListReports()))

	// Initialize security
	nonceTracker := security.NewNonceTracker(config.securityConfig.Nonce)
	defer nonceTracker.Stop()

	// Initialize query logging
	queryLogger := logging.NewQueryLogger(config.queryLogging, config.queryLogDir)
	defer queryLogger.Close()
	
	if config.queryLogging {
		log.Printf("📝 Query logging enabled: %s", config.queryLogDir)
	}

	// Initialize database
	dbManager, err := database.NewManagerWithLogger(config.databasesConfig, queryLogger)
	if err != nil {
		log.Fatalf("Failed to initialize database manager: %v", err)
	}
	defer dbManager.CloseAll()

	// Create handlers
	embedHandler := handler.NewEmbedHandler(
		reportLoader, dbManager, nonceTracker,
		config.hmacSecret, config.enablePublicPaths,
		config.allowOrigins, config.allowedCDNs,
		config.securityConfig,
	)
	refreshHandler := handler.NewRefreshHandler(
		reportLoader, dbManager, nonceTracker,
		config.hmacSecret, config.enablePublicPaths,
		config.allowOrigins,
		config.securityConfig,
	)

	// Pass snippets to handlers (they will use them during query execution)
	embedHandler.SetSnippets(config.snippets)
	refreshHandler.SetSnippets(config.snippets)

	// Log security mode
	if config.enablePublicPaths {
		log.Printf("⚠️  PUBLIC PATHS ENABLED - security validation bypassed")
	} else {
		log.Printf("🔒 Security enabled (HMAC, nonce, expiry validation)")
	}
	
	// Log security config
	log.Printf("🔒 Security: URL expiry [%s - %s], default %s", 
		config.securityConfig.Expiry.Min, config.securityConfig.Expiry.Max, config.securityConfig.Expiry.Default)
	log.Printf("🔒 Security: Nonce %d bytes (%s), max age %s, max uses %d",
		config.securityConfig.Nonce.Bytes, config.securityConfig.Nonce.Encoding,
		config.securityConfig.Nonce.MaxAge, config.securityConfig.Nonce.MaxUses)
	log.Printf("🔒 Security: Refresh grace %s", config.securityConfig.RefreshGrace)
	
	// Log CSP configuration
	if len(config.allowOrigins) == 1 && config.allowOrigins[0] == "*" {
		log.Printf("🌐 CSP frame-ancestors: * (any origin can embed)")
	} else if len(config.allowOrigins) == 0 {
		log.Printf("🌐 CSP frame-ancestors: 'self' (same-origin only)")
	} else {
		log.Printf("🌐 CSP frame-ancestors: 'self' %s", strings.Join(config.allowOrigins, " "))
	}
	
	// Log allowed CDNs
	if len(config.allowedCDNs) > 0 {
		log.Printf("📦 Allowed CDNs: %s", strings.Join(config.allowedCDNs, ", "))
	}

	// Start server
	srv := server.New(embedHandler, refreshHandler, config.port, config.staticDir)
	log.Printf("Server starting on port %s", config.port)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

type config struct {
	port            string
	hmacSecret      []byte
	reportsDir      string
	staticDir       string
	databasesConfig string
	enablePublicPaths bool
	allowOrigins    []string
	allowedCDNs     []string
	queryLogging    bool
	queryLogDir     string
	securityConfig  core.SecurityConfig
	snippets        map[string]*loader.Snippet
}

func loadConfig() *config {
	cfg := &config{}

	// Port
	if port := os.Getenv("PORT"); port != "" {
		cfg.port = port
	} else {
		cfg.port = "8080"
	}

	// HMAC secret (required)
	if secret := os.Getenv("HMAC_SECRET"); secret != "" {
		cfg.hmacSecret = []byte(secret)
	} else {
		log.Fatal("HMAC_SECRET must be set (non-empty)")
	}

	// Allow origins for CSP frame‑ancestors
	if origins := os.Getenv("ALLOW_ORIGINS"); origins != "" {
		if origins == "*" {
			cfg.allowOrigins = []string{"*"}
		} else {
			split := strings.Split(origins, ",")
			cfg.allowOrigins = make([]string, len(split))
			for i, o := range split {
				cfg.allowOrigins[i] = strings.TrimSpace(o)
			}
		}
	} else {
		cfg.allowOrigins = []string{"*"}  // default: allow any origin
	}

	// Reports directory
	if dir := os.Getenv("REPORTS_DIR"); dir != "" {
		cfg.reportsDir = dir
	} else {
		cfg.reportsDir = "./reports"
	}

	// Static directory
	if dir := os.Getenv("STATIC_DIR"); dir != "" {
		cfg.staticDir = dir
	} else {
		cfg.staticDir = "./static"
	}

	// Database config
	if path := os.Getenv("DATABASES_CONFIG"); path != "" {
		cfg.databasesConfig = path
	} else {
		cfg.databasesConfig = "./databases.yaml"
	}

	// Enable public paths (for testing)
	if enable := os.Getenv("ENABLE_PUBLIC_PATHS"); enable != "" {
		cfg.enablePublicPaths = (enable == "true" || enable == "1")
	} else {
		cfg.enablePublicPaths = false
	}

	// Allowed CDNs for CSP
	if cdns := os.Getenv("ALLOWED_CDNS"); cdns != "" {
		cdnsList := strings.Split(cdns, ",")
		cfg.allowedCDNs = make([]string, len(cdnsList))
		for i, cdn := range cdnsList {
			cfg.allowedCDNs[i] = strings.TrimSpace(cdn)
		}
	} else {
		cfg.allowedCDNs = []string{}
	}

	// Query logging
	if logging := os.Getenv("QUERY_LOGGING"); logging != "" {
		cfg.queryLogging = (logging == "true" || logging == "1")
	} else {
		cfg.queryLogging = false
	}

	if dir := os.Getenv("QUERY_LOG_DIR"); dir != "" {
		cfg.queryLogDir = dir
	} else {
		cfg.queryLogDir = "./query_log"
	}

	// Load default security config and override with env vars
	cfg.securityConfig = core.DefaultSecurityConfig()
	cfg.securityConfig = parseSecurityConfig(cfg.securityConfig)

	// Validate security config
	if err := cfg.securityConfig.Validate(); err != nil {
		log.Fatalf("Invalid security configuration: %v", err)
	}

	// Load snippets (optional)
	snippetsDir := os.Getenv("SNIPPETS_DIR")
	if snippetsDir == "" {
		snippetsDir = "./snippets"
	}
	cfg.snippets = nil
	if info, err := os.Stat(snippetsDir); err == nil && info.IsDir() {
		cfg.snippets, err = loader.LoadSnippets(snippetsDir)
		if err != nil {
			log.Printf("⚠️  Failed to load snippets from %s: %v (snippets are optional)", snippetsDir, err)
		} else {
			log.Printf("📦 Loaded %d snippets from %s", len(cfg.snippets), snippetsDir)
		}
	} else {
		log.Printf("ℹ️  No snippets directory found at %s (snippets are optional)", snippetsDir)
	}

	return cfg
}

// parseSecurityConfig reads Phase 1 env vars and overrides defaults.
func parseSecurityConfig(cfg core.SecurityConfig) core.SecurityConfig {
	// URL expiry
	if v := os.Getenv("URL_EXPIRY_DEFAULT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Expiry.Default = d
		}
	}
	if v := os.Getenv("URL_EXPIRY_MIN"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Expiry.Min = d
		}
	}
	if v := os.Getenv("URL_EXPIRY_MAX"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Expiry.Max = d
		}
	}
	if v := os.Getenv("REFRESH_GRACE_PERIOD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RefreshGrace = d
		}
	}

	// Nonce settings
	if v := os.Getenv("NONCE_BYTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Nonce.Bytes = n
		}
	}
	if v := os.Getenv("NONCE_ENCODING"); v != "" {
		cfg.Nonce.Encoding = v
	}
	if v := os.Getenv("NONCE_MAX_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Nonce.MaxAge = d
		}
	}
	if v := os.Getenv("NONCE_CLEANUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Nonce.CleanupInterval = d
		}
	}
	if v := os.Getenv("NONCE_MAX_USES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Nonce.MaxUses = n
		}
	}
	if v := os.Getenv("NONCE_USE_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Nonce.UseWindow = d
		}
	}

	return cfg
}

// generateURL generates a signed URL for testing.
func generateURL(reportID string, expiresSec int, paramsStr string, secCfg core.SecurityConfig) {
	if reportID == "" {
		log.Fatal("Report ID required (use -report flag)")
	}

	// Load reports to validate
	loader, err := loader.New("./reports")
	if err != nil {
		log.Fatalf("Failed to load reports: %v", err)
	}

	report, err := loader.GetReport(reportID)
	if err != nil {
		log.Fatalf("Report not found: %v", err)
	}

	// Parse parameters
	params := make(map[string][]string)
	if paramsStr != "" {
		// Parse parameters: key=value pairs separated by commas
		// Values can contain commas, so we need smarter parsing
		// We'll parse sequentially looking for key=value patterns
		
		// First, let's handle a simpler approach for now: require explicit format
		// key=val1,val2,val3:key2=val4,val5
		// Use : as separator between key-value pairs
		// This is backward compatible? Actually old format was comma-separated pairs
		// Let's support both: if we find : in the string, use it as pair separator
		// Otherwise, fall back to old behavior (comma-separated pairs, no multi-value)
		
		if strings.Contains(paramsStr, "|") {
			// New format with | as pair separator
			pairs := strings.Split(paramsStr, "|")
			for _, pair := range pairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) != 2 {
					log.Fatalf("Invalid parameter format: %s (expected key=value)", pair)
				}
				key := strings.TrimSpace(kv[0])
				valueStr := strings.TrimSpace(kv[1])
				
				// Split value by commas
				if valueStr == "" {
					params[key] = []string{""}
				} else {
					rawValues := strings.Split(valueStr, ",")
					values := make([]string, len(rawValues))
					for i, v := range rawValues {
						values[i] = strings.TrimSpace(v)
					}
					params[key] = values
				}
			}
		} else {
			// Old format: comma-separated key=value pairs (no multi-value support)
			// For backward compatibility during transition
			pairs := strings.Split(paramsStr, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) != 2 {
					log.Fatalf("Invalid parameter format: %s (expected key=value)", pair)
				}
				key := strings.TrimSpace(kv[0])
				valueStr := strings.TrimSpace(kv[1])
				params[key] = []string{valueStr}
			}
		}
	}

	// Validate parameters
	if err := report.ValidateParams(params); err != nil {
		log.Fatalf("Parameter validation failed: %v", err)
	}

	// Extract immutable parameters
	immutableParams := report.ExtractImmutable(params)

	// Get HMAC secret from environment
	secret := os.Getenv("HMAC_SECRET")
	if secret == "" {
		log.Fatal("HMAC_SECRET must be set in environment")
	}

	// Generate nonce using the shared helper
	nonce, err := security.GenerateNonce(secCfg.Nonce.Bytes, secCfg.Nonce.Encoding)
	if err != nil {
		log.Fatalf("Failed to generate nonce: %v", err)
	}

	// Calculate expires
	expires := time.Now().Unix() + int64(expiresSec)

	// Validate expires against config bounds
	if err := security.ValidateExpiry(time.Duration(expiresSec)*time.Second, 
		secCfg.Expiry.Min, secCfg.Expiry.Max); err != nil {
		log.Fatalf("Invalid -expires value: %v", err)
	}

	// Sign URL
	sig := security.SignURL(reportID, expires, nonce, immutableParams, []byte(secret))

	// Build URL - use configurable port if available
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	finalURL := fmt.Sprintf("http://localhost:%s/api/embed?report_id=%s&expires=%d&nonce=%s&sig=%s",
		port, reportID, expires, nonce, sig)

	// Add all parameters (including empty values for optional mutable parameters)
	for key, values := range params {
		for _, value := range values {
			// Always include parameter, even if empty
			finalURL += fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(value))
		}
	}

	fmt.Println(finalURL)
}