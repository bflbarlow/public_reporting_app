# Customer SQL Database Documentation

## Overview

This database contains anonymized and non-anonymized data from findhelp customer sites, supporting reporting on social determinants of health (SDOH), referrals, assessments, and user activity. The data is organized into several thematic areas, each with related tables.

## Database Schema Overview

### Reporting Areas and Tables

| Reporting Area | Tables | Purpose |
|----------------|--------|---------|
| **Forms/Assessments** | `assessments`, `goals`, `form_responses` | Stores questions and responses from forms/assessments, goal tracking, and program/screener form submissions. |
| **Referrals** | `referrals`, `referral_status`, `flexible_referral_fields`, `notes`, `note_options` | Tracks referral lifecycle, status history, custom referral fields, and notes attached to goals/referrals. |
| **Program Data** | `programs` | Contains program information (services, providers, categories) referenced in referrals. |
| **User Demographics** | `seeker_profiles`, `seeker_profile_custom_values`, `seeker_profile_audit_log`, `user_external_identifiers`, `seeker_context`, `seeker_households` | Stores seeker (client) demographic data, custom fields, audit logs, external IDs, session context, and household information. |
| **Worker Data** | `worker_profiles`, `worker_groups`, `worker_group_assignment_history` | Contains data about workers (navigators) including profiles, group assignments, and audit history. |
| **Site Actions/Auditing** | `user_activity` | Tracks all user activity on the site (searches, interactions, logins, etc.). |
| **User Favorites** | `user_favorites` | Programs favorited by users. |
| **Organization-Defined Data** | `sdoh_code_mappings` | Maps LOINC, Z-Codes, or other code sets to findhelp entities (flexible referral fields, assessments). |
| **Messaging Data** | `seeker_sms_messages` | Text messages between workers and seekers (if SMS enabled). |
| **Metadata** | `vw_etl_metadata` | ETL metadata for auditing and quality assurance. |

## Key Tables and Relationships

### Core Entities

1. **Seekers (Clients)**
   - Central table: `seeker_profiles` (`seeker_id`, `seeker_profile_id`)
   - Related: `seeker_profile_custom_values`, `seeker_households`, `seeker_context`
   - Join to: `referrals.seeker_id`, `referral_status.seeker_id`, `assessments.seeker_id`, `goals.seeker_id`

2. **Referrals**
   - Central table: `referrals` (`referral_id`, `referral_key`, `referral_numeric_id`)
   - Status history: `referral_status` (multiple rows per referral)
   - Custom fields: `flexible_referral_fields`
   - Join to programs: `referrals.program_numeric_id` = `programs.program_numeric_id`
   - Join to forms: `referrals.form_submission_id` = `assessments.form_submission_id` or `form_responses.form_submission_id`

3. **Forms/Assessments**
   - Two main tables: `assessments` (general assessments) and `form_responses` (program/screener forms)
   - Both share similar structure: `form_submission_id`, `question`, `answer`, etc.
   - Join to seekers: `seeker_id`, `seeker_profile_id`
   - Join to referrals: `form_submission_id`

4. **Programs**
   - `programs` table: `program_numeric_id`, `provider_numeric_id`, categories, service tags
   - Join to referrals and user_favorites via `program_numeric_id`

5. **Workers (Navigators)**
   - `worker_profiles` (`user_id`)
   - Group membership: `worker_groups`, `worker_group_assignment_history`
   - Join to referrals: `referrals.sender_user_id` = `worker_profiles.user_id`
   - Join to notes: `notes.author_user_id`
   - Join to goals: `goals.author_user_id`

### Key Join Patterns

```sql
-- Referrals with latest status
SELECT r.*, rs.status, rs.status_date
FROM referrals r
LEFT JOIN (
    SELECT referral_id, status, status_date,
           ROW_NUMBER() OVER (PARTITION BY referral_id ORDER BY status_date DESC) AS rn
    FROM referral_status
) rs ON r.referral_id = rs.referral_id AND rs.rn = 1

-- Referrals with program details
SELECT r.*, p.program_name, p.provider_name, p.categories
FROM referrals r
LEFT JOIN programs p ON r.program_numeric_id = p.program_numeric_id

-- Seeker with custom fields
SELECT sp.*, spcv.field_name, spcv.value
FROM seeker_profiles sp
LEFT JOIN seeker_profile_custom_values spcv ON sp.seeker_profile_id = spcv.seeker_profile_id

-- Assessments with seeker info
SELECT a.*, sp.first_name, sp.last_name
FROM assessments a
LEFT JOIN seeker_profiles sp ON a.seeker_id = sp.seeker_id

-- Activity with seeker info
SELECT ua.*, sp.first_name, sp.last_name
FROM user_activity ua
LEFT JOIN seeker_profiles sp ON ua.user_id = sp.seeker_id
```

## Important Columns and Data Types

### Common Identifier Columns

| Column Name | Description | Typical Data Type |
|-------------|-------------|-------------------|
| `seeker_id` | Numeric identifier for seekers (clients) | INT |
| `seeker_profile_id` | Profile ID for a seeker within a specific subdomain | INT |
| `user_id` | Numeric identifier for workers (navigators) | INT |
| `referral_id` | Numeric identifier for referrals | INT |
| `referral_key` | Unique alphanumeric identifier for referrals | VARCHAR(100) |
| `program_numeric_id` | Numeric identifier for programs | BIGINT |
| `form_submission_id` | ID for form submissions | INT |
| `subdomain` | Subdomain filter for multi-tenant data | VARCHAR(45-100) |

### Temporal Columns

| Column Name | Description | Typical Data Type |
|-------------|-------------|-------------------|
| `entry_date` | When the row was inserted into the reporting database | TIMESTAMP |
| `update_date` | When the row was last updated (often equals entry_date) | TIMESTAMP |
| `created_at`, `started_at`, `referral_date` | Business event timestamps | DATETIME |
| `status_date`, `activity_entry_date` | Status/activity timestamps | DATETIME |

### PII/PHI Considerations

Columns marked as "May Contain PII/PHI? = Yes" in source documentation contain potentially sensitive data. These include:
- `seeker_profiles`: `email`, `username`, `phone_number`, `first_name`, `last_name`
- `seeker_households`: `first_name`, `last_name`, `phone_number`, `email`, `date_of_birth`
- `worker_profiles`: `first_name`, `last_name`, `registered_email`, `phone_number`
- `assessments`/`form_responses`: `answer` (free-text responses)
- `notes`: `note_text`
- `user_activity`: `activity_data`, `ip_address`
- `user_external_identifiers`: `external_identifier`

## Common Reporting Scenarios

### 1. Referral Funnel Analysis
```sql
-- Count referrals by status over time
SELECT 
    DATE(referral_date) as referral_day,
    referral_status,
    COUNT(*) as referral_count
FROM referrals
GROUP BY DATE(referral_date), referral_status
ORDER BY referral_day DESC, referral_status;
```

### 2. Seeker Engagement
```sql
-- Seekers with multiple referrals
SELECT 
    seeker_id,
    COUNT(DISTINCT referral_id) as referral_count,
    MIN(referral_date) as first_referral,
    MAX(referral_date) as last_referral
FROM referrals
GROUP BY seeker_id
HAVING COUNT(DISTINCT referral_id) > 1;
```

### 3. Program Performance
```sql
-- Most referred programs
SELECT 
    p.program_name,
    p.provider_name,
    COUNT(r.referral_id) as referral_count,
    AVG(CASE WHEN r.referral_status = 'successful' THEN 1 ELSE 0 END) as success_rate
FROM referrals r
JOIN programs p ON r.program_numeric_id = p.program_numeric_id
GROUP BY p.program_name, p.provider_name
ORDER BY referral_count DESC;
```

### 4. Assessment Responses Analysis
```sql
-- Common answers to specific questions
SELECT 
    question,
    answer,
    COUNT(*) as response_count
FROM assessments
WHERE question LIKE '%food%'
GROUP BY question, answer
ORDER BY response_count DESC;
```

### 5. Worker Activity
```sql
-- Worker referral volume
SELECT 
    wp.first_name,
    wp.last_name,
    COUNT(r.referral_id) as referrals_sent,
    COUNT(DISTINCT r.seeker_id) as unique_seekers
FROM referrals r
JOIN worker_profiles wp ON r.sender_user_id = wp.user_id
GROUP BY wp.first_name, wp.last_name
ORDER BY referrals_sent DESC;
```

## Data Quality Notes

1. **Subdomain Filtering**: Always consider filtering by `subdomain` if your organization uses multiple subdomains.
2. **Historical Data**: For referral status history, use `referral_status` table; `referrals.referral_status` contains only the latest status.
3. **PII Handling**: Be mindful of PII/PHI columns when sharing or exporting data.
4. **NULL Values**: Some demographic fields may be NULL depending on customer workflows.
5. **ETL Timing**: Check `vw_etl_metadata` for table load times and freshness.
6. **Data Granularity**: `assessments` and `form_responses` store one row per question-response pair, not per form submission.

## Table Quick Reference

| Table | Primary Key | Key Columns | Key Relationships |
|-------|-------------|-------------|-------------------|
| `assessments` | `id_assessments` | `form_submission_id`, `seeker_id`, `seeker_profile_id` | `referrals.form_submission_id`, `seeker_profiles.seeker_id` |
| `form_responses` | `id_form_responses` | `form_submission_id`, `seeker_id`, `seeker_profile_id` | `referrals.form_submission_id`, `seeker_profiles.seeker_id` |
| `referrals` | `id_referrals` | `referral_id`, `referral_key`, `seeker_id`, `program_numeric_id` | `referral_status.referral_id`, `programs.program_numeric_id`, `seeker_profiles.seeker_id` |
| `referral_status` | `id_referral_status` | `referral_id`, `status_date` | `referrals.referral_id` |
| `programs` | `id_programs` | `program_numeric_id`, `provider_numeric_id` | `referrals.program_numeric_id`, `user_favorites.program_numeric_id` |
| `seeker_profiles` | `id_customer_seeker_profiles` | `seeker_id`, `seeker_profile_id` | Most tables with `seeker_id` or `seeker_profile_id` |
| `worker_profiles` | `id_worker_profiles` | `user_id` | `referrals.sender_user_id`, `notes.author_user_id`, `goals.author_user_id` |
| `user_activity` | `id_user_activity` | `user_id`, `activity_entry_date`, `session_id` | `seeker_profiles.seeker_id` (if user_id maps to seeker) |
| `goals` | `id_goals` | `goal_id`, `seeker_id`, `seeker_profile_id` | `seeker_profiles.seeker_id`, `notes.note_type_id` (when note_type='GoalNote') |
| `notes` | `id_notes` | `note_id`, `seeker_id`, `note_type`, `note_type_id` | `goals.goal_id` (when note_type='GoalNote'), `referrals.referral_id` (when note_type='ReferralNote') |

## Support and Resources

- For detailed column-level documentation, refer to the source Excel documentation.
- The `vw_etl_metadata` view provides information about data freshness and ETL processes.
- Always test queries with appropriate filters (date ranges, subdomains) before production use.

---

*Documentation generated from customer SQL database schema. Last updated: May 2026.*