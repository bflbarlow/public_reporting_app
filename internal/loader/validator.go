package loader

import (
	"fmt"
	"regexp"
	"strings"
	
	"reporting_app/internal/core"
)

// ValidateReport validates a report definition
func ValidateReport(report *core.Report) error {
	if report.ID == "" {
		return fmt.Errorf("report ID is required")
	}
	
	if report.Name == "" {
		return fmt.Errorf("report name is required")
	}
	
	if report.Database == "" {
		return fmt.Errorf("database is required")
	}
	
	if report.Visibility != core.VisibilityPublic && report.Visibility != core.VisibilityPrivate {
		return fmt.Errorf("visibility must be 'public' or 'private'")
	}
	
	if len(report.Datasources) == 0 {
		return fmt.Errorf("at least one datasource is required")
	}
	
	// Validate parameter names don't conflict with system params
	systemParams := map[string]bool{
		core.ParamReportID: true,
		core.ParamExpires:  true,
		core.ParamNonce:    true,
		core.ParamSig:      true,
	}
	
	for _, name := range report.ImmutableParams {
		if systemParams[name] {
			return fmt.Errorf("parameter name '%s' conflicts with system parameter", name)
		}
		if !isValidParamName(name) {
			return fmt.Errorf("invalid parameter name '%s': must be alphanumeric with underscores", name)
		}
	}
	
	for _, name := range report.MutableParams {
		if systemParams[name] {
			return fmt.Errorf("parameter name '%s' conflicts with system parameter", name)
		}
		if !isValidParamName(name) {
			return fmt.Errorf("invalid parameter name '%s': must be alphanumeric with underscores", name)
		}
	}
	
	// Validate datasources
	for name, ds := range report.Datasources {
		if err := ValidateDatasource(name, ds, report); err != nil {
			return err
		}
	}
	
	return nil
}

// ValidateDatasource validates a single datasource
func ValidateDatasource(name string, ds core.Datasource, report *core.Report) error {
	if ds.SQL == "" {
		return fmt.Errorf("datasource %s: sql is required", name)
	}
	
	// Extract parameter placeholders from SQL
	params := extractSQLParams(ds.SQL)
	
	// Validate all referenced parameters are declared
	for _, param := range params {
		if !report.ContainsParam(param) {
			return fmt.Errorf("datasource %s: SQL references undeclared parameter '%s'", name, param)
		}
	}
	
	return nil
}

// extractSQLParams extracts {{param_name}} placeholders from SQL
func extractSQLParams(sql string) []string {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(sql, -1)
	
	params := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	
	for _, match := range matches {
		param := match[1]
		if !seen[param] {
			seen[param] = true
			params = append(params, param)
		}
	}
	
	return params
}

// isValidParamName validates parameter name format
func isValidParamName(name string) bool {
	// Allow alphanumeric and underscores
	re := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	return re.MatchString(name)
}

// InjectParams injects parameters into SQL, replacing {{param}} with ?
func InjectParams(sql string, params map[string]string) (string, []interface{}) {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	
	var result strings.Builder
	result.Grow(len(sql))
	var args []interface{}
	
	lastIndex := 0
	for _, match := range re.FindAllStringSubmatchIndex(sql, -1) {
		// Copy text before match
		result.WriteString(sql[lastIndex:match[0]])
		
		// Extract parameter name
		paramName := sql[match[2]:match[3]]
		
		// Get value from params
		if value, ok := params[paramName]; ok {
			result.WriteString("?")
			args = append(args, value)
		} else {
			// Keep placeholder if value not provided
			result.WriteString("{{" + paramName + "}}")
		}
		
		lastIndex = match[1]
	}
	
	// Copy remaining text
	result.WriteString(sql[lastIndex:])
	
	return result.String(), args
}