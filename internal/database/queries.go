package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"reporting_app/internal/core"
	"reporting_app/internal/loader"
	"reporting_app/internal/logging"
)

// ExecuteQuery executes a SQL query with parameters and returns results
func ExecuteQuery(db *sql.DB, sql string, args []interface{}, rowLimit int, timeout time.Duration) (*core.QueryResult, error) {
	return ExecuteQueryWithLogger(db, sql, args, rowLimit, timeout, "", "", "", nil)
}

// ExecuteQueryWithLogger executes a SQL query with parameters and returns results
func ExecuteQueryWithLogger(db *sql.DB, sql string, args []interface{}, rowLimit int, timeout time.Duration, reportID, datasource, dbName string, logger *logging.QueryLogger) (*core.QueryResult, error) {
	return ExecuteQueryWithLoggerAndReport(db, sql, args, rowLimit, timeout, reportID, datasource, dbName, "", logger)
}

// ExecuteQueryWithLoggerAndReport executes a SQL query with parameters and returns results,
// also accepting the report name for logging.
func ExecuteQueryWithLoggerAndReport(db *sql.DB, sql string, args []interface{}, rowLimit int, timeout time.Duration, reportID, datasource, dbName, reportName string, logger *logging.QueryLogger) (*core.QueryResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, sql, args...)
	duration := time.Since(start)

	var resultRows [][]interface{}
	var columns []string
	var rowCount int

	if err == nil {
		defer rows.Close()

		columns, err = rows.Columns()
		if err != nil {
			err = fmt.Errorf("failed to get columns: %w", err)
		} else {
			count := 0

			for rows.Next() {
				if count >= rowLimit {
					err = fmt.Errorf("row_limit exceeded: %d", rowLimit)
					break
				}

				values := make([]interface{}, len(columns))
				valuePtrs := make([]interface{}, len(columns))
				for i := range values {
					valuePtrs[i] = &values[i]
				}

				if scanErr := rows.Scan(valuePtrs...); scanErr != nil {
					err = fmt.Errorf("row scan failed: %w", scanErr)
					break
				}

				// Convert []byte to string for JSON compatibility
				for i, val := range values {
					if b, ok := val.([]byte); ok {
						values[i] = string(b)
					}
				}

				resultRows = append(resultRows, values)
				count++
			}

			if err == nil {
				err = rows.Err()
			}
			rowCount = len(resultRows)
		}
	}

	// Log the query if logger is provided
	if logger != nil {
		// Generate the actual SQL with parameters substituted for readability
		actualSQL := formatActualSQL(sql, args)
		dateDir := time.Now().UTC().Format("2006-01-02")
		timestamp := time.Now().UTC().Format("15_04_05")
		logger.Log(dateDir, timestamp, datasource, reportID, reportName, dbName, actualSQL, duration, rowCount, rowLimit, err)
	}

	if err != nil {
		return nil, err
	}

	return &core.QueryResult{Columns: columns, Rows: resultRows}, nil
}

// ExecuteDatasource executes a datasource query for a report
func ExecuteDatasource(db *sql.DB, ds core.Datasource, params map[string][]string, report *core.Report, defaultRowLimit int, timeout time.Duration) (*core.QueryResult, error) {
	return ExecuteDatasourceWithLogger(db, ds, params, report, defaultRowLimit, timeout, "", "", "", nil)
}

// ExecuteDatasourceWithLogger executes a datasource query for a report with logging
func ExecuteDatasourceWithLogger(db *sql.DB, ds core.Datasource, params map[string][]string, report *core.Report, defaultRowLimit int, timeout time.Duration, reportID, datasource, dbName string, logger *logging.QueryLogger) (*core.QueryResult, error) {
	// Determine row limit
	rowLimit := defaultRowLimit
	if ds.RowLimit > 0 && ds.RowLimit < rowLimit {
		rowLimit = ds.RowLimit
	}

	// Inject parameters into SQL
	sql, args := InjectParams(ds.SQL, params, report)

	return ExecuteQueryWithLoggerAndReport(db, sql, args, rowLimit, timeout, reportID, datasource, dbName, report.Name, logger)
}

// InjectParams is a wrapper around loader.InjectParams
// Placed here for database package convenience
func InjectParams(sql string, params map[string][]string, report *core.Report) (string, []interface{}) {
	return loader.InjectParams(sql, params, report)
}

// formatActualSQL substitutes parameter placeholders with their actual values
// for human-readable logging output.
func formatActualSQL(sql string, args []interface{}) string {
	result := sql
	for _, arg := range args {
		// Replace the first ? with the formatted argument
		idx := -1
		for i := 0; i < len(result); i++ {
			if result[i] == '?' {
				idx = i
				break
			}
		}
		if idx == -1 {
			break
		}

		var replacement string
		if arg == nil {
			replacement = "NULL"
		} else {
			switch v := arg.(type) {
			case string:
				replacement = fmt.Sprintf("'%s'", escapeSQLString(v))
			case int, int8, int16, int32, int64:
				replacement = fmt.Sprintf("%d", v)
			case uint, uint8, uint16, uint32, uint64:
				replacement = fmt.Sprintf("%d", v)
			case float32, float64:
				replacement = fmt.Sprintf("%g", v)
			case bool:
				if v {
					replacement = "TRUE"
				} else {
					replacement = "FALSE"
				}
			default:
				replacement = fmt.Sprintf("'%v'", v)
			}
		}

		result = result[:idx] + replacement + result[idx+1:]
	}
	return result
}

// escapeSQLString escapes single quotes in a string for safe display.
func escapeSQLString(s string) string {
	result := []byte{}
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			result = append(result, '\'')
			result = append(result, '\'')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}
