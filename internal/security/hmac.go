package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"reporting_app/internal/core"
)

// SignURL signs a URL with HMAC using only immutable parameters
func SignURL(reportID string, expires int64, nonce string, immutableParams map[string][]string, secret []byte) string {
	// Build message: report_id:expires:nonce:canonical_immutable_params
	// where canonical_immutable_params is key=urlencode(value)&key2=urlencode(value2)
	// with keys sorted alphabetically
	canonical := canonicalParams(immutableParams)
	message := fmt.Sprintf("%s:%d:%s:%s",
		reportID,
		expires,
		nonce,
		canonical)
	
	return signMessage(message, secret)
}

// canonicalParams creates a canonical string representation of parameters
// Keys are sorted alphabetically, values are URL-encoded and pipe-separated
func canonicalParams(params map[string][]string) string {
	if len(params) == 0 {
		return ""
	}
	
	// Collect and sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	// Build canonical string
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteString("&")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		// Sort values within each key for determinism
		values := make([]string, len(params[k]))
		copy(values, params[k])
		sort.Strings(values)
		for j, v := range values {
			if j > 0 {
				buf.WriteString("|")
			}
			buf.WriteString(url.QueryEscape(v))
		}
	}
	return buf.String()
}

// VerifyURL verifies a URL signature
func VerifyURL(reportID string, expires int64, nonce string, immutableParams map[string][]string, sig string, secret []byte) bool {
	expected := SignURL(reportID, expires, nonce, immutableParams, secret)
	return hmac.Equal([]byte(expected), []byte(sig))
}

// signMessage creates HMAC signature
func signMessage(message string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// ExtractParams extracts all parameters (except HMAC params) from query string
// Returns all values for each parameter to support multi-value parameters
func ExtractParams(query url.Values) map[string][]string {
	params := make(map[string][]string)
	
	for key, values := range query {
		// Skip HMAC-related parameters
		if key == core.ParamReportID || key == core.ParamExpires || 
		   key == core.ParamNonce || key == core.ParamSig {
			continue
		}
		
		if len(values) > 0 {
			// Include all values for multi-value support
			params[key] = values
		} else {
			// Parameter without value (key with no =) becomes slice with empty string
			params[key] = []string{""}
		}
	}
	
	return params
}

// ParseSignedParams extracts HMAC parameters from query string
func ParseSignedParams(query url.Values) (reportID string, expires int64, nonce, sig string, err error) {
	reportID = query.Get(core.ParamReportID)
	if reportID == "" {
		return "", 0, "", "", fmt.Errorf("missing %s", core.ParamReportID)
	}
	
	expiresStr := query.Get(core.ParamExpires)
	if expiresStr == "" {
		return "", 0, "", "", fmt.Errorf("missing %s", core.ParamExpires)
	}
	
	// Parse expires as int64
	_, err = fmt.Sscanf(expiresStr, "%d", &expires)
	if err != nil {
		return "", 0, "", "", fmt.Errorf("invalid %s: %w", core.ParamExpires, err)
	}
	
	nonce = query.Get(core.ParamNonce)
	if nonce == "" {
		return "", 0, "", "", fmt.Errorf("missing %s", core.ParamNonce)
	}
	
	sig = query.Get(core.ParamSig)
	if sig == "" {
		return "", 0, "", "", fmt.Errorf("missing %s", core.ParamSig)
	}
	
	return reportID, expires, nonce, sig, nil
}

// ValidateExpiry checks that a duration is within the allowed range.
func ValidateExpiry(d time.Duration, min, max time.Duration) error {
	if d < min {
		return fmt.Errorf("expiry %s is below minimum %s", d, min)
	}
	if d > max {
		return fmt.Errorf("expiry %s exceeds maximum %s", d, max)
	}
	return nil
}

// GenerateNonce generates a cryptographically random nonce.
func GenerateNonce(bytes int, encoding string) (string, error) {
	if bytes < 16 || bytes > 64 {
		return "", fmt.Errorf("nonce bytes must be between 16 and 64, got %d", bytes)
	}
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return EncodeNonce(raw, encoding), nil
}

// EncodeNonce encodes raw bytes using the specified encoding.
func EncodeNonce(raw []byte, encoding string) string {
	switch encoding {
	case "hex":
		return hex.EncodeToString(raw)
	case "base64":
		return base64.StdEncoding.EncodeToString(raw)
	case "urlsafe-base64", "":
		return base64.URLEncoding.EncodeToString(raw)
	default:
		return base64.URLEncoding.EncodeToString(raw)
	}
}