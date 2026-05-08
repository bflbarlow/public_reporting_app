package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogEntry represents a single query log entry
type LogEntry struct {
	Timestamp  string        `json:"timestamp"`
	ReportID   string        `json:"report_id"`
	Datasource string        `json:"datasource,omitempty"`
	Database   string        `json:"database"`
	SQL        string        `json:"sql"`
	Params     []interface{} `json:"params"`
	DurationMs float64       `json:"duration_ms"`
	RowCount   int           `json:"row_count"`
	Error      string        `json:"error,omitempty"`
	RowLimit   int           `json:"row_limit,omitempty"`
}

// QueryLogger handles logging of database queries
type QueryLogger struct {
	enabled bool
	dir     string
	mu      sync.Mutex
	file    *os.File
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
		
		if err := logger.openLogFile(); err != nil {
			log.Printf("Failed to open query log file: %v", err)
			logger.enabled = false
		} else {
			log.Printf("Query logging enabled: %s", dir)
		}
	}
	
	return logger
}

// ensureDirectory creates the log directory if it doesn't exist
func (ql *QueryLogger) ensureDirectory() error {
	return os.MkdirAll(ql.dir, 0755)
}

// openLogFile opens or creates the daily log file
func (ql *QueryLogger) openLogFile() error {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	
	// Close existing file if open
	if ql.file != nil {
		ql.file.Close()
		ql.file = nil
	}
	
	// Create filename based on current date
	filename := filepath.Join(ql.dir, time.Now().Format("2006-01-02")+".log")
	
	// Open file for appending
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", filename, err)
	}
	
	ql.file = file
	return nil
}

// getCurrentLogFile ensures we're writing to the correct daily file
func (ql *QueryLogger) getCurrentLogFile() (*os.File, error) {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	
	if ql.file == nil {
		if err := ql.openLogFile(); err != nil {
			return nil, err
		}
	}
	
	// Check if we need to rotate to a new day
	currentFilename := filepath.Join(ql.dir, time.Now().Format("2006-01-02")+".log")
	actualFilename := ql.file.Name()
	
	if currentFilename != actualFilename {
		// Day changed, open new file
		if err := ql.openLogFile(); err != nil {
			return nil, err
		}
	}
	
	return ql.file, nil
}

// Log writes a query log entry
func (ql *QueryLogger) Log(reportID, datasource, dbName, sql string, params []interface{}, duration time.Duration, rows int, rowLimit int, queryErr error) {
	if !ql.enabled {
		return
	}
	
	// Prepare error message
	errorMsg := ""
	if queryErr != nil {
		errorMsg = queryErr.Error()
	}
	
	// Create log entry
	entry := LogEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		ReportID:   reportID,
		Datasource: datasource,
		Database:   dbName,
		SQL:        sql,
		Params:     params,
		DurationMs: float64(duration.Microseconds()) / 1000.0, // Convert to milliseconds
		RowCount:   rows,
		Error:      errorMsg,
		RowLimit:   rowLimit,
	}
	
	// Marshal to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal query log entry: %v", err)
		return
	}
	
	// Get current log file
	file, err := ql.getCurrentLogFile()
	if err != nil {
		log.Printf("Failed to get query log file: %v", err)
		return
	}
	
	// Write entry with newline
	ql.mu.Lock()
	defer ql.mu.Unlock()
	
	if _, err := file.Write(append(jsonData, '\n')); err != nil {
		log.Printf("Failed to write query log entry: %v", err)
	}
}

// Close closes the log file
func (ql *QueryLogger) Close() error {
	ql.mu.Lock()
	defer ql.mu.Unlock()
	
	if ql.file != nil {
		return ql.file.Close()
	}
	return nil
}