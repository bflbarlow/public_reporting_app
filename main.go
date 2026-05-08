package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"reporting_app/internal/database"
	"reporting_app/internal/handler"
	"reporting_app/internal/loader"
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
	paramsStr := flag.String("params", "", "Parameters as key=value,key=value")
	flag.Parse()

	if *genURL {
		generateURL(*reportID, *expiresSec, *paramsStr)
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
	nonceTracker := security.NewNonceTracker(60 * time.Second)
	defer nonceTracker.Stop()

	// Initialize database
	dbManager, err := database.NewManager(config.databasesConfig)
	if err != nil {
		log.Fatalf("Failed to initialize database manager: %v", err)
	}
	defer dbManager.CloseAll()

	// Create handlers
	embedHandler := handler.NewEmbedHandler(reportLoader, dbManager, nonceTracker, config.hmacSecret, config.enablePublicPaths, config.allowOrigins, config.allowedCDNs)
	refreshHandler := handler.NewRefreshHandler(reportLoader, dbManager, nonceTracker, config.hmacSecret, config.enablePublicPaths, config.allowOrigins)

	// Log security mode
	if config.enablePublicPaths {
		log.Printf("⚠️  PUBLIC PATHS ENABLED - security validation bypassed")
	} else {
		log.Printf("🔒 Security enabled (HMAC, nonce, expiry validation)")
	}
	
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

	return cfg
}

// generateURL generates a signed URL for testing
func generateURL(reportID string, expiresSec int, paramsStr string) {
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
	params := make(map[string]string)
	if paramsStr != "" {
		pairs := strings.Split(paramsStr, ",")
		for _, pair := range pairs {
			kv := strings.Split(pair, "=")
			if len(kv) != 2 {
				log.Fatalf("Invalid parameter format: %s (expected key=value)", pair)
			}
			params[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	// Validate parameters
	if err := report.ValidateParams(params); err != nil {
		log.Fatalf("Parameter validation failed: %v", err)
	}

	// Generate nonce
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		log.Fatalf("Failed to generate nonce: %v", err)
	}
	nonce := base64.URLEncoding.EncodeToString(nonceBytes)

	// Calculate expires
	expires := time.Now().Unix() + int64(expiresSec)

	// Extract immutable parameters
	immutableParams := report.ExtractImmutable(params)

	// Get HMAC secret from environment
	secret := os.Getenv("HMAC_SECRET")
	if secret == "" {
		log.Fatal("HMAC_SECRET must be set in environment")
	}

	// Sign URL
	sig := security.SignURL(reportID, expires, nonce, immutableParams, []byte(secret))

	// Build URL
	finalURL := fmt.Sprintf("http://localhost:8080/api/embed?report_id=%s&expires=%d&nonce=%s&sig=%s",
		reportID, expires, nonce, sig)

	// Add all parameters
	for key, value := range params {
		if value != "" {
			finalURL += fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(value))
		}
	}

	fmt.Println(finalURL)
}