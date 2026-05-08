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
		logger.Log(reportID, datasource, dbName, sql, args, duration, rowCount, rowLimit, err)
	}
	
	if err != nil {
		return nil, err
	}
	
	return &core.QueryResult{Columns: columns, Rows: resultRows}, nil
}

// ExecuteDatasource executes a datasource query for a report
func ExecuteDatasource(db *sql.DB, ds core.Datasource, params map[string]string, defaultRowLimit int, timeout time.Duration) (*core.QueryResult, error) {
	return ExecuteDatasourceWithLogger(db, ds, params, defaultRowLimit, timeout, "", "", "", nil)
}

// ExecuteDatasourceWithLogger executes a datasource query for a report with logging
func ExecuteDatasourceWithLogger(db *sql.DB, ds core.Datasource, params map[string]string, defaultRowLimit int, timeout time.Duration, reportID, datasource, dbName string, logger *logging.QueryLogger) (*core.QueryResult, error) {
	// Determine row limit
	rowLimit := defaultRowLimit
	if ds.RowLimit > 0 && ds.RowLimit < rowLimit {
		rowLimit = ds.RowLimit
	}
	
	// Inject parameters into SQL
	sql, args := InjectParams(ds.SQL, params)
	
	return ExecuteQueryWithLogger(db, sql, args, rowLimit, timeout, reportID, datasource, dbName, logger)
}

// InjectParams is a wrapper around loader.InjectParams
// Placed here for database package convenience
func InjectParams(sql string, params map[string]string) (string, []interface{}) {
	return loader.InjectParams(sql, params)
}