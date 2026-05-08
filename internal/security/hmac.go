package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"
	
	"reporting_app/internal/core"
)

// SignURL signs a URL with HMAC using only immutable parameters
func SignURL(reportID string, expires int64, nonce string, immutableParams map[string]string, secret []byte) string {
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
// Keys are sorted alphabetically, values are URL-encoded
func canonicalParams(params map[string]string) string {
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
		buf.WriteString(url.QueryEscape(params[k]))
	}
	return buf.String()
}

// VerifyURL verifies a URL signature
func VerifyURL(reportID string, expires int64, nonce string, immutableParams map[string]string, sig string, secret []byte) bool {
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
// Includes parameters even with empty values to support optional mutable parameters
func ExtractParams(query url.Values) map[string]string {
	params := make(map[string]string)
	
	for key, values := range query {
		// Skip HMAC-related parameters
		if key == core.ParamReportID || key == core.ParamExpires || 
		   key == core.ParamNonce || key == core.ParamSig {
			continue
		}
		
		if len(values) > 0 {
			// Include parameter even if value is empty string
			params[key] = values[0]
		} else {
			// Parameter without value (key with no =) becomes empty string
			params[key] = ""
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