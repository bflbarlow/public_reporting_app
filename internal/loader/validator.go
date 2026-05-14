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

	// Validate multi_value_params references
	allDeclared := make(map[string]bool)
	for _, n := range report.ImmutableParams {
		allDeclared[n] = true
	}
	for _, n := range report.MutableParams {
		allDeclared[n] = true
	}

	for _, n := range report.MultiValueParams {
		if !allDeclared[n] {
			return fmt.Errorf("multi_value_params references undeclared parameter '%s'", n)
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
// Now supports default values: {{param:'default'}}, {{param:-10}}, {{param:1,2,3}}
func extractSQLParams(sql string) []string {
	re := regexp.MustCompile(`\{\{(\w+)(?::([^}]+))?\}\}`)
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
// Supports multi-value parameters and default values
func InjectParams(sql string, params map[string][]string, report *core.Report) (string, []interface{}) {
	// Regex captures parameter name and optional default clause.
	// Default clause can be a single quoted string, a numeric literal, or comma‑separated values.
	re := regexp.MustCompile(`\{\{(\w+)(?::([^}]+))?\}\}`)

	var result strings.Builder
	result.Grow(len(sql))
	var args []interface{}

	lastIndex := 0
	for _, match := range re.FindAllStringSubmatchIndex(sql, -1) {
		result.WriteString(sql[lastIndex:match[0]])

		paramName := sql[match[2]:match[3]]
		defaultClause := ""
		if match[4] != -1 {
			defaultClause = sql[match[4]:match[5]]
		}

		// Determine values to use
		var values []string
		var fromDefaults bool
		if vals, ok := params[paramName]; ok && vals != nil {
			// Parameter present in URL
			// Check if all values are empty strings (i.e., `param=`)
			allEmpty := true
			for _, v := range vals {
				if v != "" {
					allEmpty = false
					break
				}
			}
			if allEmpty && defaultClause != "" {
				// Empty parameter (`param=`) → treat as absent, use default
				values = parseDefaultClause(defaultClause)
				fromDefaults = true
			} else {
				// Non‑empty values present
				values = vals
			}
		} else if defaultClause != "" {
			// Parameter absent, use default
			values = parseDefaultClause(defaultClause)
			fromDefaults = true
		}
		// If values is nil → will expand to NULL

		// Determine if parameter supports multiple values
		isMulti := report.IsMultiValue(paramName)

		// Expand placeholder
		if len(values) == 0 {
			// Missing or empty → NULL
			result.WriteString("NULL")
			// No arg added
		} else if len(values) == 1 {
			// Single value
			result.WriteString("?")
			// If the value came from defaults or is non‑empty, use it
			// Empty values from defaults shouldn't happen (parseDefaultClause returns nil for empty clause)
			if values[0] == "" && !fromDefaults {
				// Empty value from URL (should have been caught above as allEmpty)
				args = append(args, nil)
			} else {
				args = append(args, values[0])
			}
		} else {
			// Multiple values
			if !isMulti {
				// Parameter not declared multi‑value: take first value only
				// (Could also raise an error at validation time)
				result.WriteString("?")
				if values[0] == "" && !fromDefaults {
					args = append(args, nil)
				} else {
					args = append(args, values[0])
				}
			} else {
				// Multi‑value: expand to comma‑separated placeholders
				placeholders := make([]string, len(values))
				for i, v := range values {
					placeholders[i] = "?"
					if v == "" && !fromDefaults {
						// Empty value from URL (should have been caught above)
						args = append(args, nil)
					} else {
						args = append(args, v)
					}
				}
				result.WriteString(strings.Join(placeholders, ","))
			}
		}

		lastIndex = match[1]
	}

	result.WriteString(sql[lastIndex:])
	return result.String(), args
}

// parseDefaultClause parses a default clause like "'default'", "-10", "1,2,3", or "'a','b','c'".
// It returns a slice of strings (already stripped of quotes).
// Note: Default values in the SQL placeholder use commas as separators (e.g., `{{param:1,2,3}}`),
// but the HMAC canonical form uses pipes for value separation.
func parseDefaultClause(clause string) []string {
	clause = strings.TrimSpace(clause)
	if clause == "" {
		return nil
	}

	// Split by commas, respecting quoted strings
	// Simple implementation: split on commas, trim spaces, strip surrounding single quotes
	var values []string
	// This simple parser assumes no embedded commas or quotes inside values.
	// For production, use a proper CSV parser or a more robust tokenizer.
	parts := strings.Split(clause, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Remove surrounding single quotes if present
		if len(part) >= 2 && part[0] == '\'' && part[len(part)-1] == '\'' {
			part = part[1 : len(part)-1]
			// Unescape doubled single quotes
			part = strings.ReplaceAll(part, "''", "'")
		}
		values = append(values, part)
	}
	return values
}