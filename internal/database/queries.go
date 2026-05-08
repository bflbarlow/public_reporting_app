package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	
	"reporting_app/internal/core"
	"reporting_app/internal/loader"
)

// ExecuteQuery executes a SQL query with parameters and returns results
func ExecuteQuery(db *sql.DB, sql string, args []interface{}, rowLimit int, timeout time.Duration) (*core.QueryResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var resultRows [][]interface{}
	count := 0
	
	for rows.Next() {
		if count >= rowLimit {
			return nil, fmt.Errorf("row_limit exceeded: %d", rowLimit)
		}
		
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
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
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	
	return &core.QueryResult{Columns: columns, Rows: resultRows}, nil
}

// ExecuteDatasource executes a datasource query for a report
func ExecuteDatasource(db *sql.DB, ds core.Datasource, params map[string]string, defaultRowLimit int, timeout time.Duration) (*core.QueryResult, error) {
	// Determine row limit
	rowLimit := defaultRowLimit
	if ds.RowLimit > 0 && ds.RowLimit < rowLimit {
		rowLimit = ds.RowLimit
	}
	
	// Inject parameters into SQL
	sql, args := InjectParams(ds.SQL, params)
	
	return ExecuteQuery(db, sql, args, rowLimit, timeout)
}

// InjectParams is a wrapper around loader.InjectParams
// Placed here for database package convenience
func InjectParams(sql string, params map[string]string) (string, []interface{}) {
	return loader.InjectParams(sql, params)
}