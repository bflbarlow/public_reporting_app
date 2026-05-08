package security

import "strings"

// GenerateCSPHeader generates a Content-Security-Policy header for a report
func GenerateCSPHeader(allowOrigins []string, additionalScripts []string) string {
	directives := []string{
		// Default policy: block everything
		"default-src 'none'",
		
		// Allow scripts from self and thick client
		"script-src 'self' 'unsafe-inline' 'unsafe-eval'",
		
		// Allow connections to self (for thick client refresh)
		"connect-src 'self'",
		
		// Allow styles from self
		"style-src 'self' 'unsafe-inline'",
		
		// Allow images from self and data URLs
		"img-src 'self' data:",
		
		// Allow fonts from self
		"font-src 'self'",
		
		// Form actions can only go to same origin
		"form-action 'self'",
		
		// Frame ancestors (configurable)
		GenerateFrameAncestors(allowOrigins),
		
		// Other directives for security
		"base-uri 'self'",
		"object-src 'none'",
	}
	
	// Note: allowOrigins are used only for frame-ancestors, not script-src
	
	// Add additional script sources (CDNs)
	if len(additionalScripts) > 0 {
		// Add to script-src for loading scripts
		for i, dir := range directives {
			if strings.HasPrefix(dir, "script-src") {
				for _, src := range additionalScripts {
					directives[i] += " " + src
				}
				break
			}
		}
		
		// Also add to connect-src for source maps and other connections
		for i, dir := range directives {
			if strings.HasPrefix(dir, "connect-src") {
				for _, src := range additionalScripts {
					directives[i] += " " + src
				}
				break
			}
		}
	}
	
	return strings.Join(directives, "; ")
}

// GenerateFrameAncestors generates frame-ancestors directive for embedding
func GenerateFrameAncestors(allowOrigins []string) string {
	// Always start with 'self' for same‑origin embedding
	frameAncestors := []string{"'self'"}
	
	for _, origin := range allowOrigins {
		if origin == "*" {
			frameAncestors = append(frameAncestors, "*")
		} else if origin != "" {
			frameAncestors = append(frameAncestors, origin)
		}
	}
	
	return "frame-ancestors " + strings.Join(frameAncestors, " ")
}