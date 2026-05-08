package loader

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	
	"reporting_app/internal/core"
	"gopkg.in/yaml.v3"
)

// Loader discovers and loads reports from a directory
type Loader struct {
	reportsDir string
	reports    map[string]*core.Report
}

// New creates a new loader and loads all reports
func New(reportsDir string) (*Loader, error) {
	l := &Loader{
		reportsDir: reportsDir,
		reports:    make(map[string]*core.Report),
	}
	
	if err := l.Reload(); err != nil {
		return nil, err
	}
	
	return l, nil
}

// Reload walks the reports directory and loads all reports
func (l *Loader) Reload() error {
	l.reports = make(map[string]*core.Report)
	
	return filepath.WalkDir(l.reportsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if !d.IsDir() {
			return nil
		}
		
		// Skip the root reports dir
		if path == l.reportsDir {
			return nil
		}
		
		reportID := filepath.Base(path)
		// Look for report.yaml, fall back to manifest.yaml
		reportPath := filepath.Join(path, "report.yaml")
		if _, err := os.Stat(reportPath); os.IsNotExist(err) {
			reportPath = filepath.Join(path, "manifest.yaml")
			if _, err := os.Stat(reportPath); os.IsNotExist(err) {
				log.Printf("Report %s missing report.yaml/manifest.yaml, skipping", reportID)
				return fs.SkipDir
			}
		}
		
		report, err := parseReportYAML(reportPath)
		if err != nil {
			log.Printf("Failed to parse report %s: %v", reportID, err)
			return fs.SkipDir
		}
		
		// Ensure ID matches directory name
		if report.ID == "" {
			report.ID = reportID
		}
		
		l.reports[report.ID] = report
		log.Printf("Loaded report: %s (%s)", report.ID, report.Name)
		
		return fs.SkipDir // Don't descend into subdirectories
	})
}

// GetReport returns a report by ID
func (l *Loader) GetReport(id string) (*core.Report, error) {
	report, ok := l.reports[id]
	if !ok {
		return nil, fmt.Errorf("report not found: %s", id)
	}
	return report, nil
}

// ListReports returns all loaded report IDs
func (l *Loader) ListReports() []string {
	ids := make([]string, 0, len(l.reports))
	for id := range l.reports {
		ids = append(ids, id)
	}
	return ids
}

// parseReportYAML parses a report.yaml file
func parseReportYAML(path string) (*core.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read report file %s: %w", path, err)
	}
	
	// Parse YAML into intermediate struct
	var raw struct {
		ID              string                 `yaml:"id"`
		Name            string                 `yaml:"name"`
		Description     string                 `yaml:"description"`
		Database        string                 `yaml:"database"`
		Visibility      string                 `yaml:"visibility"`
		ExpiresAfter    int                    `yaml:"expires_after"`
		MaxRows         int                    `yaml:"max_rows"`
		ImmutableParams interface{}            `yaml:"immutable_params"`
		MutableParams   interface{}            `yaml:"mutable_params"`
		Datasources     map[string]interface{} `yaml:"datasources"`
	}
	
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	// Convert parameters from various formats to []string
	convertParams := func(v interface{}) []string {
		if v == nil {
			return []string{}
		}
		switch val := v.(type) {
		case []interface{}:
			result := make([]string, len(val))
			for i, item := range val {
				result[i] = fmt.Sprintf("%v", item)
			}
			return result
		case []string:
			return val
		case map[string]interface{}:
			result := make([]string, 0, len(val))
			for key := range val {
				result = append(result, key)
			}
			return result
		case map[interface{}]interface{}:
			result := make([]string, 0, len(val))
			for key := range val {
				result = append(result, fmt.Sprintf("%v", key))
			}
			return result
		default:
			// Unknown format, return empty
			return []string{}
		}
	}
	
	immutableParams := convertParams(raw.ImmutableParams)
	mutableParams := convertParams(raw.MutableParams)
	
	// Parse datasources
	datasources := make(map[string]core.Datasource)
	for name, ds := range raw.Datasources {
		parsed, err := parseDatasource(ds)
		if err != nil {
			return nil, fmt.Errorf("datasource %s: %w", name, err)
		}
		datasources[name] = parsed
	}
	
	// Create report
	report := &core.Report{
		ID:              raw.ID,
		Name:            raw.Name,
		Description:     raw.Description,
		Database:        raw.Database,
		Visibility:      raw.Visibility,
		ExpiresAfter:    raw.ExpiresAfter,
		MaxRows:         raw.MaxRows,
		ImmutableParams: immutableParams,
		MutableParams:   mutableParams,
		Datasources:     datasources,
	}
	
	// Set defaults
	if report.Visibility == "" {
		report.Visibility = core.VisibilityPublic
	}
	if report.ExpiresAfter == 0 {
		report.ExpiresAfter = core.DefaultExpiry
	}
	if report.MaxRows == 0 {
		report.MaxRows = core.DefaultRowLimit
	}
	
	return report, nil
}

// parseDatasource parses a datasource from YAML
func parseDatasource(data interface{}) (core.Datasource, error) {
	var ds core.Datasource
	
	// Try to parse as map
	if m, ok := data.(map[string]interface{}); ok {
		if sql, ok := m["sql"].(string); ok {
			ds.SQL = sql
		} else {
			return ds, fmt.Errorf("missing required field: sql")
		}
		
		if rowLimit, ok := m["row_limit"].(int); ok {
			ds.RowLimit = rowLimit
		}
		
		if cacheTTL, ok := m["cache_ttl"].(int); ok {
			ds.CacheTTL = cacheTTL
		}
		
		return ds, nil
	}
	
	return ds, fmt.Errorf("datasource must be a map with sql field")
}