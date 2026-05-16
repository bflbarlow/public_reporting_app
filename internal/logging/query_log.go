package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// QueryLogger handles logging of database queries.
// Each query is written as a separate text file for easy reading.
type QueryLogger struct {
	enabled bool
	dir     string
	mu      sync.Mutex
}

// NewQueryLogger creates a new query logger
func NewQueryLogger(enabled bool, dir string) *QueryLogger {
	logger := &QueryLogger{
		enabled: enabled,
		dir:     dir,
	}

	if enabled {
		if err := logger.ensureDirectory(); err != nil {
			log.Printf("Failed to create query log directory: %v", err)
			logger.enabled = false
			return logger
		}
		log.Printf("Query logging enabled: %s", dir)
	}

	return logger
}

// ensureDirectory creates the log directory if it doesn't exist
func (ql *QueryLogger) ensureDirectory() error {
	return os.MkdirAll(ql.dir, 0755)
}

// ensureDateDirectory creates a date subdirectory if it doesn't exist
func (ql *QueryLogger) ensureDateDirectory(date string) error {
	dir := filepath.Join(ql.dir, date)
	return os.MkdirAll(dir, 0755)
}

// sanitizeFilename removes characters that are invalid in filenames
func sanitizeFilename(s string) string {
	result := s
	result = replaceAll(result, ":", "_")
	result = replaceAll(result, "/", "_")
	result = replaceAll(result, "\\", "_")
	result = replaceAll(result, "*", "_")
	result = replaceAll(result, "?", "_")
	result = replaceAll(result, "\"", "_")
	result = replaceAll(result, "<", "_")
	result = replaceAll(result, ">", "_")
	result = replaceAll(result, "|", "_")
	return result
}

// replaceAll is a simple string replacement helper
func replaceAll(s, old, new string) string {
	result := []byte{}
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result = append(result, []byte(new)...)
			i += len(old) - 1
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

// Log writes a query log entry as a separate text file.
// Parameters:
//
//	dateDir: date directory (e.g. "2026-05-15")
//	timestamp: UTC timestamp for the filename
//	datasource: the datasource name
//	reportID: the report ID
//	reportName: the human-readable report name
//	dbName: the database name
//	actualSQL: the actual SQL sent to the database (with parameters substituted)
//	duration: query execution duration
//	rows: number of rows returned
//	rowLimit: the row limit that was applied
//	queryErr: any error that occurred
func (ql *QueryLogger) Log(dateDir, timestamp, datasource, reportID, reportName, dbName, actualSQL string, duration time.Duration, rows int, rowLimit int, queryErr error) {
	if !ql.enabled {
		return
	}

	// Prepare error message
	errorMsg := ""
	if queryErr != nil {
		errorMsg = queryErr.Error()
	}

	// Build filename: YYYY-MM-DDTHH_MM_SSZ_datasource_report
	safeDS := sanitizeFilename(datasource)
	safeReport := sanitizeFilename(reportID)
	filename := fmt.Sprintf("%sT%s_%s_%s.txt", dateDir, timestamp, safeDS, safeReport)

	// Build file path
	filePath := filepath.Join(ql.dir, dateDir, filename)

	// Ensure date directory exists
	if err := ql.ensureDateDirectory(dateDir); err != nil {
		log.Printf("Failed to create query log date directory %s: %v", dateDir, err)
		return
	}

	// Format duration
	durationStr := fmt.Sprintf("%.3fms", float64(duration.Microseconds())/1000.0)

	// Build content
	var content string
	if queryErr != nil {
		content = fmt.Sprintf(
			"--- %s UTC ---\nreport: %s\ndatasource: %s\ndatabase: %s\nduration: %s\nrows: %d (ERROR)\nrow_limit: %d\n---\n\n%s\n\nERROR: %s\n",
			timestamp, reportName, datasource, dbName,
			durationStr, rows, rowLimit,
			actualSQL, errorMsg,
		)
	} else {
		content = fmt.Sprintf(
			"--- %s UTC ---\nreport: %s\ndatasource: %s\ndatabase: %s\nduration: %s\nrows: %d\nrow_limit: %d\n---\n\n%s\n",
			timestamp, reportName, datasource, dbName,
			durationStr, rows, rowLimit,
			actualSQL,
		)
	}

	// Write file
	ql.mu.Lock()
	defer ql.mu.Unlock()

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		log.Printf("Failed to write query log file %s: %v", filePath, err)
	}
}

// Close is a no-op for the file-per-query logger
func (ql *QueryLogger) Close() error {
	return nil
}
