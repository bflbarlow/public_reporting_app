# Multi-Select Parameters: Design Flaws & Implementation Issues

## Executive Summary

The multi-select parameter enhancement, as implemented in the `InjectParams` function (`internal/loader/validator.go`), contains a fundamental design flaw that causes SQL syntax errors when parameters are used outside of comparison contexts. The most visible manifestation is the failure of the `referral_funnel_dashboard` report with error:

```
Error 1064 (42000): You have an error in your SQL syntax; check the manual that corresponds to your MySQL server version for the right syntax to use near '= ? AND = ?
GROUP BY r.referral_id, p.program_name
ORDER BY days_to_latest_statu' at line 16
```

## Technical Problem

### The Core Issue: Overly Assumptive Parameter Expansion

The `InjectParams` function assumes that every `{{parameter}}` placeholder appears in a comparison context where a comparison operator is required. This assumption breaks when parameters appear in other SQL contexts.

**Current expansion logic:**
- 0 values → expands to `1=1`
- 1 value → expands to `= ?`  
- 2+ values → expands to `IN (?, ?, ...)`

**Problematic SQL Contexts:**
1. **BETWEEN clauses**: `BETWEEN {{start_date}} AND {{end_date}}` → `BETWEEN = ? AND = ?` (invalid)
2. **Function arguments**: `COALESCE({{param}}, default)` → `COALESCE(= ?, default)` (invalid)
3. **Mathematical expressions**: `value + {{addend}}` → `value + = ?` (invalid)

## Impact Analysis

### Directly Affected Reports

1. **`referral_funnel_dashboard`** - failing with `BETWEEN` syntax error
2. **All reports using `BETWEEN` with parameter placeholders**: 
   - `program_analysis_v2`
   - `customer_program_insights`
   - `program_analysis`
   - `customer_sql_reporting`
3. **Reports using parameters in function calls**:
   - `customer_referral_analytics` (uses `COALESCE({{param}}, default)`)
   - `customer_seeker_analytics` (uses `COALESCE({{param}}, default)`)

### Hidden Issues

1. **Intermittent failures**: Reports may work when parameters have values but fail when empty
2. **Syntax errors masked by COALESCE**: Attempted "fix" creates `COALESCE(= ?, default)` (still invalid)
3. **Template documentation mismatch**: Report template suggests `COALESCE()` pattern but system doesn't support it

## Root Cause Analysis

### 1. Design Philosophy Conflict
The parameter system was designed for filters (`WHERE status = {{status}}`) but is being used for value injection (`BETWEEN {{start_date}} AND {{end_date}}`).

### 2. Missing Syntax Context Awareness
The parser doesn't differentiate between:
- Comparison contexts: `WHERE column {{param}}`
- Value contexts: `BETWEEN {{param}} AND {{param2}}`
- Expression contexts: `value + {{param}}`

### 3. Mode System Incomplete
While the system added mode specifiers (`:in`, `:eq`, `:value`), they are:
- Not documented in report template
- Not required for correctness
- Not automatically applied to existing reports
- `:value` mode was added as a reaction, not part of original design

## Code-Level Analysis

### Current `InjectParams` Implementation Flaws

```go
// Line 173-226 in validator.go

// Default mode (no :mode specified) assumes comparison
switch mode {
case "in":
    // Always expand as IN clause
    // ...
case "eq":
    // Always expand as single = ?
    // ...
case "value":
    // Expand as just a value placeholder ? (no operator)
    // This mode was added AFTER the problem was found
    // ...
default:
    // Auto mode: expand based on value count
    // This is where the problem occurs:
    // - 0 values → "1=1" (breaks BETWEEN)
    // - 1 value → "= ?" (breaks BETWEEN, COALESCE)  
    // - 2+ values → "IN (?, ?, ...)" (breaks BETWEEN)
}
```

### SQL Grammar Incompatibility

**Valid SQL:**
```sql
WHERE date BETWEEN ? AND ?
WHERE value = COALESCE(?, default)
WHERE id IN (?, ?, ?)
```

**Current Expansion Produces:**
```sql
WHERE date BETWEEN = ? AND = ?          -- Invalid: extra "="
WHERE value = COALESCE(= ?, default)    -- Invalid: extra "="
WHERE id IN (?, ?, ?)                   -- Valid ONLY in :in mode
```

## The ":value" Mode Problematic Solution

The recently added `:value` mode creates new problems:

1. **Report Migration Burden**: All existing reports must be updated
2. **Backward Compatibility Broken**: Old reports fail silently or with errors
3. **Inconsistent State**: Some reports may work (no params), some fail (with params)
4. **Documentation Gap**: No guidance on when to use which mode

## Security Implications

1. **Error Leakage**: SQL syntax errors exposed in API responses
2. **Denial of Service**: Malicious parameters could trigger SQL errors
3. **Inconsistent Behavior**: Different parameter values produce different SQL constructs
4. **Audit Trail Corruption**: Failed queries may not be logged correctly

## Broader System Impact

### 1. Report Template System Unusable
The recommended pattern in `report_template/report.yaml`:
```sql
WHERE created_at BETWEEN {{start_date}} AND {{end_date}}
WHERE ({{status}} IS NULL OR status = {{status}})
```
These patterns are fundamentally incompatible with the current implementation.

### 2. Multi-Value vs Single-Value Confusion
The system conflates:
- Multi-value parameters (arrays for IN clauses)
- Comparison context parameters (need operators)
- Value context parameters (no operators)

### 3. Parameter Semantics Mismatch
```sql
-- Intended meaning: Filter by optional status
WHERE ({{status}} IS NULL OR status = {{status}})

-- Actual expansion: Syntax error when status has 0 or 1 value
WHERE (1=1 OR status = = ?)      -- 0 values: incorrect
WHERE (= ? IS NULL OR status = = ?) -- 1 value: double "="
```

## Recommended Solutions

### Immediate (Workaround)
1. **Add `:value` mode to all non-comparison parameters**:
   ```sql
   WHERE date BETWEEN {{start_date:value}} AND {{end_date:value}}
   WHERE value = COALESCE({{param:value}}, default)
   ```

### Short-term (Patch)
1. **Context-aware parser**: Detect SQL context before adding operators
2. **Smart default mode**: Use syntax analysis to choose appropriate mode
3. **Validation warnings**: Flag problematic patterns at report load time

### Long-term (Redesign)
1. **Explicit mode-required design**: All parameters must specify usage mode
2. **Separate syntax categories**: Value params, comparison params, expression params
3. **SQL-aware templating**: Template engine understands SQL grammar

## Conclusion

The multi-select parameter enhancement introduced an architectural flaw by assuming all parameters appear in comparison contexts. This breaks fundamental SQL patterns like `BETWEEN`, `COALESCE()`, and mathematical expressions. The current workaround (`:value` mode) shifts the burden to report authors and requires updating all existing reports.

**Urgent Action Required**: 
- Audit all reports for affected patterns
- Document required mode specifiers
- Consider rollback or significant redesign

**Impact Assessment**: 
- High risk of production failures
- Moderate remediation effort
- High technical debt accumulation

---
*Document generated: 2026-05-10*
*Based on analysis of referral_funnel_dashboard failure and code review of validator.go*