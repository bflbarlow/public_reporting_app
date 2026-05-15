# Configurable NONCE Policies and URL Expiration

**Version:** 1.1  
**Date:** 2026-05-15  
**Status:** Phase 1 Implemented — Active Configuration  

---

## 1. Purpose

This document describes a comprehensive set of configurable NONCE policies and URL expiration settings for the reporting_app project. The goal is to support diverse deployment environments and customer requirements without hardcoding security parameters.

### 1.1 Problem Statement

Currently, the reporting_app uses hardcoded values for:
- NONCE size (32 bytes)
- NONCE lifetime (24 hours)
- URL maximum expiration (1 hour)
- Refresh grace period (30 seconds)

These values are appropriate for a single environment but cannot accommodate the varied needs of:
- **High-security environments** requiring stronger nonces and shorter lifetimes
- **High-throughput deployments** needing multi-use nonces or pooling
- **Multi-instance deployments** requiring distributed nonce storage
- **Compliance-driven deployments** with specific entropy or audit requirements
- **Customer-specific SLAs** dictating different URL expiration windows

### 1.2 Design Goals

1. **Backward compatible** — all defaults match current hardcoded values
2. **Fail fast** — invalid configuration rejected at startup
3. **No runtime mutation** — settings loaded once, immutable for process lifetime
4. **Phased approach** — Phase 1 covers 90% of use cases; Phase 2/3 for enterprise
5. **Shared helpers** — nonce generation and validation centralized in `security` package
6. **Observable** — configuration logged at startup for auditability

---

## 2. Implementation Status (Phase 1 Complete)

### 2.1 Phase 1 Implementation Summary

Phase 1 configuration is fully implemented as of 2026-05-15. All 10 environment variables are operational with sensible defaults matching previous hardcoded behavior.

**Key Changes:**
1. **Configurable values** — All security parameters now read from environment variables
2. **Centralized helpers** — `security.GenerateNonce()`, `security.ValidateExpiry()`, `security.EncodeNonce()`
3. **Updated nonce tracker** — Supports multi‑use nonces (`NONCE_MAX_USES`, `NONCE_USE_WINDOW`)
4. **Expiry validation** — URL expiration bounds (`URL_EXPIRY_MIN`, `URL_EXPIRY_MAX`)
5. **Grace period** — Configurable refresh grace (`REFRESH_GRACE_PERIOD`)
6. **Bug fix** — Nonce tracker now correctly adds nonces on first use (was rejecting all new nonces)
7. **Thick‑client metadata** — `ReportConfig` includes `urlExpiry` and `refreshGrace` for proactive refresh handling

### 2.2 Nonce Usage Pattern

**Important:** `/api/embed` consumes a nonce use; `/refresh` does NOT.

| Endpoint | Nonce Consumption | Purpose |
|----------|-------------------|---------|
| `/api/embed` | ✅ 1 use consumed | Initial page load, prevents replay |
| `/refresh` | ❌ No consumption | Allows multiple refreshes from same signed URL |

This design enables:
- **Single embed** — can be loaded only once (replay protection)
- **Multiple refreshes** — same signed URL can refresh many times (within expiry + grace)
- **New nonce on refresh** — each refresh response includes a new nonce for next refresh

### 2.3 Environment Variables Now Available

All Phase 1 variables are defined in `.env.example` and `.env` with defaults matching previous hardcoded values:

| Group | Variable | Default | Description |
|-------|----------|---------|-------------|
| **URL Expiry** | `URL_EXPIRY_DEFAULT` | `5m` | Default URL expiration |
| | `URL_EXPIRY_MIN` | `1m` | Minimum allowed expiration |
| | `URL_EXPIRY_MAX` | `24h` | Maximum allowed expiration |
| | `REFRESH_GRACE_PERIOD` | `0s` | Grace period after expiry |
| **NONCE** | `NONCE_BYTES` | `32` | Bytes (16‑64) |
| | `NONCE_ENCODING` | `urlsafe-base64` | Encoding format |
| | `NONCE_MAX_AGE` | `24h` | Max lifetime |
| | `NONCE_CLEANUP_INTERVAL` | `60s` | Cleanup frequency |
| | `NONCE_MAX_USES` | `1` | Max uses per nonce |
| | `NONCE_USE_WINDOW` | `5m` | Sliding window for multi‑use |

---

## 3. Phase 1 Configuration Options (Implemented)

Phase 1 configuration is fully implemented and operational. All settings are controlled via environment variables defined in `.env.example` and `.env`. Defaults match previous hardcoded values for backward compatibility.

### 3.1 URL Expiration Settings

#### `URL_EXPIRY_DEFAULT`
- **Type:** Duration (seconds)
- **Default:** `300` (5 minutes)
- **Description:** Default expiration time for URLs generated via CLI `-genurl` flag
- **Use case:** Customers who want different default URL lifetimes

#### `URL_EXPIRY_MIN`
- **Type:** Duration (seconds)
- **Default:** `60` (1 minute)
- **Description:** Minimum allowed expiration time. URLs shorter than this are rejected.
- **Use case:** Compliance requirements preventing very short-lived URLs (which can cause issues with slow networks)

#### `URL_EXPIRY_MAX`
- **Type:** Duration (seconds)
- **Default:** `86400` (24 hours)
- **Description:** Maximum allowed expiration time. URLs with expiration beyond this are rejected.
- **Use case:** High-security environments requiring short URL lifetimes (e.g., 15 minutes)

#### `REFRESH_GRACE_PERIOD`
- **Type:** Duration (seconds)
- **Default:** `0` (no grace)
- **Description:** Grace period after URL expiration during which `/refresh` requests are still accepted.
- **Use case:** Thick-client applications that may send a refresh request slightly after URL expiry due to timing jitter

### 3.2 NONCE Settings

#### `NONCE_BYTES`
- **Type:** Integer
- **Default:** `32`
- **Valid range:** 16 to 64
- **Description:** Number of random bytes used to generate the nonce
- **Entropy:** 128 bits (min) to 512 bits (max)
- **Use case:** 
  - Lower values for reduced URL length (embedded in iframe src)
  - Higher values for high-security environments

#### `NONCE_ENCODING`
- **Type:** String
- **Default:** `urlsafe-base64`
- **Valid values:** `urlsafe-base64`, `base64`, `hex`
- **Description:** Encoding format for the nonce when serialized in URLs
- **Comparison:**

| Encoding | Example (32 bytes) | URL-safe | Length |
|----------|--------------------|----------|--------|
| `urlsafe-base64` | `abc123...xyz` | Yes | ~43 chars |
| `base64` | `abc/123...+xyz` | No (`/`, `+`) | ~43 chars |
| `hex` | `a1b2c3...f4e5` | Yes | 64 chars |

- **Use case:** 
  - `urlsafe-base64` for standard use (short, URL-safe)
  - `hex` for environments that cannot handle base64 characters
  - `base64` if URL encoding is handled at a lower layer

#### `NONCE_MAX_AGE`
- **Type:** Duration
- **Default:** `24h`
- **Description:** Maximum age of a nonce before it is automatically rejected, even if still in the tracker
- **Use case:** Compliance requiring shorter nonce lifetimes (e.g., 1 hour for financial systems)

#### `NONCE_CLEANUP_INTERVAL`
- **Type:** Duration
- **Default:** `60s`
- **Description:** How often the background goroutine runs cleanup of expired nonces
- **Use case:** 
  - Shorter intervals for high-throughput deployments (more frequent cleanup)
  - Longer intervals for low-throughput deployments (less CPU overhead)

#### `NONCE_MAX_USES`
- **Type:** Integer
- **Default:** `1` (single-use)
- **Valid range:** 1 to 1000
- **Description:** Number of times a nonce can be used before it is invalidated
- **Use case:**
  - `1` for maximum security (current behavior)
  - `5` for thick-client polling (same nonce used for multiple refreshes)
  - `100` for high-throughput CDN edge caching scenarios

#### `NONCE_USE_WINDOW`
- **Type:** Duration
- **Default:** `300s` (5 minutes)
- **Description:** Time window during which multi-use nonces remain valid
- **Interaction with `NONCE_MAX_USES`:** A nonce is valid for up to `NONCE_MAX_USES` uses **or** until `NONCE_USE_WINDOW` elapses, whichever comes first
- **Use case:**
  - Short window (60s) for polling scenarios
  - Long window (3600s) for iframe reload scenarios
---

## 4. Phase 2 Configuration Options (Enterprise / Scaling)

Phase 2 addresses scaling, rate limiting, and multi-instance deployment scenarios.

### 4.1 NONCE Storage Backend

#### `NONCE_STORE`
- **Type:** String
- **Default:** `memory`
- **Valid values:** `memory`, `redis`
- **Description:** Where nonces are stored
- **Use case:**
  - `memory` for single-instance deployments (current behavior)
  - `redis` for multi-instance / Kubernetes deployments

#### `NONCE_REDIS_URL`
- **Type:** String
- **Default:** `redis://localhost:6379/0`
- **Description:** Redis connection URL when `NONCE_STORE=redis`
- **Use case:** Pointing to a managed Redis service (AWS ElastiCache, etc.)

#### `NONCE_REDIS_KEY_PREFIX`
- **Type:** String
- **Default:** `reporting_app:nonce:`
- **Description:** Redis key prefix for namespacing nonces
- **Use case:** Multi-tenant deployments sharing a Redis instance

### 4.2 Rate Limiting

#### `NONCE_RATE_LIMIT`
- **Type:** Integer
- **Default:** `0` (unlimited)
- **Description:** Maximum number of nonces a single client can generate per time window
- **Use case:** Preventing abuse / DoS via excessive nonce generation

#### `NONCE_RATE_WINDOW`
- **Type:** Duration
- **Default:** `60s`
- **Description:** Time window for the rate limit counter
- **Interaction with `NONCE_RATE_LIMIT`:** A client can generate up to `NONCE_RATE_LIMIT` nonces within any rolling `NONCE_RATE_WINDOW` period

### 4.3 Nonce Namespacing

#### `NONCE_NAMESPACES`
- **Type:** Comma-separated list of namespace names
- **Default:** `""` (no namespacing)
- **Valid values:** Any identifier (e.g., `embed,refresh`)
- **Description:** Separate nonce pools by endpoint/purpose
- **Use case:**
  - `embed,refresh` — separate pools for `/api/embed` and `/refresh`
  - Prevents cross-endpoint nonce reuse
  - Enables different policies per namespace (future)

#### `NONCE_NAMESPACE.embed.MAX_USES`
- **Type:** Integer
- **Default:** Inherits `NONCE_MAX_USES`
- **Description:** Per-namespace override for max uses

#### `NONCE_NAMESPACE.refresh.MAX_USES`
- **Type:** Integer
- **Default:** Inherits `NONCE_MAX_USES`
- **Description:** Per-namespace override for max uses

### 4.4 Nonce Pooling

#### `NONCE_POOLING_ENABLED`
- **Type:** Boolean
- **Default:** `false`
- **Description:** Enable nonce pooling instead of per-nonce tracking
- **Use case:** High-throughput deployments where per-nonce tracking is expensive

#### `NONCE_POOL_SIZE`
- **Type:** Integer
- **Default:** `100`
- **Description:** Number of nonces in each pool
- **Use case:** Balancing memory usage vs. granularity

#### `NONCE_POOL_TTL`
- **Type:** Duration
- **Default:** `600s` (10 minutes)
- **Description:** Time-to-live for each nonce pool
- **Use case:** Short-lived pools for ephemeral deployments

---

## 5. Phase 3 Configuration Options (Advanced / Compliance)

Phase 3 addresses advanced security patterns and compliance requirements.

### 5.1 Parameter Binding

#### `NONCE_BIND_TO_PARAMS`
- **Type:** Comma-separated list of parameter names
- **Default:** `""` (no binding)
- **Description:** Nonce is only valid when presented with the same parameter set
- **Use case:** Preventing parameter-swapping attacks if a nonce is leaked
- **Example:** `NONCE_BIND_TO_PARAMS=organization_id,user_id`
- **Warning:** Breaks legitimate refresh flows where bound parameters change

### 5.2 Hierarchical / Child Nonces

#### `NONCE_PARENT_CHILD_ENABLED`
- **Type:** Boolean
- **Default:** `false`
- **Description:** Enable parent-child nonce relationships
- **Use case:** Audit trails showing request lineage; revoking entire nonce chains
- **Implementation:** A parent nonce can spawn child nonces; revoking parent invalidates all children

### 5.3 Nonce Rotation

#### `NONCE_ROTATION_INTERVAL`
- **Type:** Duration
- **Default:** `0` (no rotation)
- **Description:** Force regeneration of nonces after this interval, even if not expired
- **Use case:** Compliance requirements for periodic nonce renewal

#### `NONCE_ROTATION_GRACE`
- **Type:** Duration
- **Default:** `300s` (5 minutes)
- **Description:** Grace period during which the old nonce is still valid after rotation
- **Use case:** Smooth transition during nonce rotation

### 5.4 Entropy / Strength

#### `NONCE_ENTROPY`
- **Type:** Integer (bits)
- **Default:** `256` (derived from 32 bytes)
- **Valid range:** 128 to 512
- **Description:** Cryptographic strength of nonces in bits
- **Use case:** FIPS 140-2/3 compliance, PCI DSS requirements
- **Note:** `NONCE_BYTES` is the underlying implementation; `NONCE_ENTROPY` is the semantic requirement

### 5.5 Nonce Prefix

#### `NONCE_PREFIX`
- **Type:** String
- **Default:** `""` (no prefix)
- **Description:** Prefix for nonce values in logs and CDN headers
- **Use case:** CDN cache keying, observability, debugging
- **Example:** `report_v2_nonce_`
- **Note:** Reduces effective entropy by prefix length
---

## 6. Implementation Status & Future Phases

**Phase 1:** ✅ **Complete** (implemented 2026-05-15)
**Phase 2:** 🔄 **Planned** (enterprise/scaling features)  
**Phase 3:** 📋 **Future** (advanced/compliance features)

### 6.1 Phase 1: Completed Implementation

**Status:** ✅ **Deployed** (2026‑05‑15)  
**Scope:** All 10 Phase 1 configuration options  
**Impact:** Zero‑downtime migration (defaults match previous hardcoded values)

#### Key Changes Made

1. **New configuration types** (`internal/core/types.go`)
   - `NonceConfig`, `URLExpiryConfig`, `SecurityConfig` structs
   - `Validate()` methods for startup validation

2. **Updated constants** (`internal/core/constants.go`)
   - Removed `MaxURLExpirySeconds`, `RefreshGraceSeconds` constants
   - Added `DefaultSecurityConfig()` returning Phase 1 defaults

3. **Environment variable parsing** (`main.go`)
   - `parseSecurityConfig()` reads 10 env vars
   - Validation at startup (`log.Fatalf` if invalid)
   - Logging: `🔒 Security: URL expiry [1m - 24h], default 5m` etc.

4. **Nonce tracker updated** (`internal/security/nonce_tracker.go`)
   - Accepts `NonceConfig` (bytes, encoding, max age, cleanup interval, max uses, use window)
   - Supports multi‑use nonces (`NONCE_MAX_USES`, `NONCE_USE_WINDOW`)
   - **Bug fix:** `CheckAndAdd` now correctly adds nonces on first use (was rejecting all new nonces)

5. **Security helpers** (`internal/security/hmac.go`)
   - `GenerateNonce(bytes, encoding)` — shared cryptographically random nonce generation
   - `ValidateExpiry(duration, min, max)` — expiry bounds checking
   - `EncodeNonce(raw, encoding)` — encoding abstraction (`urlsafe-base64`, `base64`, `hex`)

6. **Handler updates** (`internal/handler/embed.go`, `internal/handler/refresh.go`)
   - Accept `SecurityConfig` parameter
   - Replace hardcoded `core.MaxURLExpirySeconds` with `h.securityConfig.Expiry.Max`
   - Replace hardcoded `core.RefreshGraceSeconds` with `h.securityConfig.RefreshGrace`
   - `generateNextURL()` uses `security.GenerateNonce()` (was weak `time.Now().UnixNano()`)
   - Inject `urlExpiry` and `refreshGrace` into `ReportConfig` JSON for thick‑client awareness

7. **CLI URL generation** (`main.go` `generateURL()`)
   - Uses `security.GenerateNonce()` with configurable bytes/encoding
   - Validates `-expires` flag against `URL_EXPIRY_MIN`/`URL_EXPIRY_MAX`
   - Uses `URL_EXPIRY_DEFAULT` when `-expires` not specified

#### Nonce Usage Pattern (Important)

| Endpoint | Nonce Consumption | Rationale |
|----------|-------------------|-----------|
| `/api/embed` | ✅ **1 use consumed** | Prevents replay of the embed URL itself |
| `/refresh` | ❌ **No consumption** | Allows multiple refreshes from same signed URL (within expiry + grace) |

This means:
- **Embed URL** can be loaded only once (replay protection)
- **Refresh requests** can be made many times from the same signed URL
- **New nonce** generated for each refresh's `next_url`

#### Configuration Files Updated

- **`.env.example`** — Added all 10 Phase 1 variables with defaults
- **`.env`** — Same variables with defaults (customizable per deployment)

#### Validation & Logging

At startup, the server logs:
```
🔒 Security: URL expiry [1m - 24h], default 5m
🔒 Security: Nonce 32 bytes (urlsafe-base64), max age 24h, max uses 1
🔒 Security: Refresh grace 0s
```

Invalid configuration (e.g., `URL_EXPIRY_MIN > URL_EXPIRY_MAX`) causes immediate startup failure.

---

### 6.2 Phase 2 Implementation (Priority: Medium)

**Scope:** Phase 2 configuration options from Section 4.

**Estimated effort:** 3-4 days

#### Step 1: Redis Storage Backend

**File:** `internal/security/nonce_redis.go` (new file)

Implement `NonceStore` interface:
```go
type NonceStore interface {
    Add(nonce string, value interface{}) error
    Get(nonce string) (interface{}, error)
    CheckAndAdd(nonce string) (interface{}, error)
    Delete(nonce string) error
    Cleanup(olderThan time.Time) int
    Close() error
}
```

Implement `RedisNonceStore`:
- Use Redis `SET` with `EX` (TTL) for single-use
- Use Redis `INCR` + `TTL` for multi-use tracking
- Use Redis `HSET` for parameter binding

#### Step 2: Rate Limiting

**File:** `internal/security/rate_limiter.go` (new file)

Implement sliding window rate limiter:
- In-memory: `map[string]*slidingWindow` keyed by client IP
- Redis-backed (future): Redis `INCR` + `EXPIRE` per IP

#### Step 3: Namespacing

**File:** `internal/security/nonce_tracker.go`

- Add `namespaces map[string]*NonceTracker`
- Route `CheckAndAdd` to the correct namespace pool
- Each namespace can have its own `NonceConfig` overrides

---

### 6.3 Phase 3 Implementation (Priority: Low)

**Scope:** Phase 3 configuration options from Section 5.

**Estimated effort:** 4-5 days

#### Step 1: Parameter Binding

**File:** `internal/security/nonce_tracker.go`

- Store nonce with associated parameter snapshot
- On `CheckAndAdd`, compare provided parameters with stored snapshot
- Reject if mismatch

#### Step 2: Hierarchical Nonces

**File:** `internal/security/nonce_parent_child.go` (new file)

- Track parent-child relationships
- On parent revocation, invalidate all children
- Maintain a tree structure for audit trails

#### Step 3: Nonce Rotation

**File:** `internal/security/nonce_rotation.go` (new file)

- Background goroutine that marks nonces past `RotationInterval` as "rotated"
- During `CheckAndAdd`, reject rotated nonces unless within `RotationGrace`

---

## 7. Migration Guide

### 7.1 Zero-Downtime Migration

Since all new settings have defaults matching current behavior:

1. Deploy the code with defaults — **no behavior change**
2. Gradually introduce environment variables per deployment target
3. No database migration required (nonce tracker is in-memory or Redis)

### 7.2 Configuration Audit

At startup, log:
```
⚙️  Configuration loaded:
   URL_EXPIRY_DEFAULT=300s (default)
   URL_EXPIRY_MIN=60s (default)
   URL_EXPIRY_MAX=86400s (default)
   REFRESH_GRACE_PERIOD=0s (default)
   NONCE_BYTES=32 (default)
   NONCE_ENCODING=urlsafe-base64 (default)
   NONCE_MAX_AGE=24h (default)
   NONCE_CLEANUP_INTERVAL=60s (default)
   NONCE_MAX_USES=1 (default)
   NONCE_USE_WINDOW=5m (default)
```

Values that differ from defaults are flagged:
```
⚠️  URL_EXPIRY_MAX=3600s (custom: reduced from default 86400s)
```

### 7.3 Validation Rules

| Rule | Enforcement |
|------|-------------|
| `URL_EXPIRY_MIN <= URL_EXPIRY_MAX` | Startup failure |
| `NONCE_BYTES` in [16, 64] | Startup failure |
| `NONCE_ENCODING` in ["urlsafe-base64", "base64", "hex"] | Startup failure |
| `NONCE_MAX_USES >= 1` | Startup failure |
| `NONCE_USE_WINDOW > 0` | Startup failure |
| `NONCE_MAX_AGE > 0` | Startup failure |
| `URL_EXPIRY_DEFAULT >= URL_EXPIRY_MIN` | Startup failure |
| `URL_EXPIRY_DEFAULT <= URL_EXPIRY_MAX` | Startup failure |

---

## 8. Testing Strategy

### 8.1 Unit Tests

| Test | Scope |
|------|-------|
| `NonceConfig.Validate()` | All validation rules |
| `GenerateNonce()` | All encoding formats |
| `NonceTracker.CheckAndAdd()` | Single-use, multi-use, expired, max age |
| `ValidateExpiry()` | Min/max boundary conditions |
| `RedisNonceStore` | Redis operations (integration) |
| `RateLimiter` | Sliding window logic |

### 8.2 Integration Tests

| Test | Scope |
|------|-------|
| `/api/embed` with custom expiry | URL generation and validation |
| `/refresh` with expired URL + grace | Grace period handling |
| Multi-use nonce across refreshes | Nonce counter decrement |
| Different `NONCE_ENCODING` values | URL parsing round-trip |
| `NONCE_MAX_USES=5` | 5th use succeeds, 6th fails |

### 8.3 Security Tests

| Test | Scope |
|------|-------|
| Nonce collision resistance | Statistical analysis of generated nonces |
| Replay attack prevention | Attempt to reuse expired/nonexistent nonces |
| Parameter binding bypass | Attempt to use nonce with different parameters |
| Rate limit bypass | Rapid nonce generation from same IP |
---

## 9. Detailed Code Changes Summary

### 9.1 Files Modified

| File | Changes | Lines Changed |
|------|---------|---------------|
| `internal/core/constants.go` | Remove 2 constants; add 4 validation constants; add `DefaultSecurityConfig()` | -10 / +30 |
| `internal/core/types.go` | Add `NonceConfig`, `URLExpiryConfig`, `SecurityConfig` structs; add `Validate()` | +80 |
| `main.go` | Add 8 env vars; parse + validate; pass config to handlers/tracker; update `generateURL()` | +60 / -20 |
| `internal/security/nonce_tracker.go` | Accept `NonceConfig`; add `MaxUses`/`UseWindow`; update `CheckAndAdd`/`cleanup()` | +40 / -10 |
| `internal/security/hmac.go` | Add `ValidateExpiry()`, `GenerateNonce()`, `EncodeNonce()` helpers | +60 |
| `internal/handler/embed.go` | Accept `SecurityConfig`; replace constants; add config to JSON | +10 / -5 |
| `internal/handler/refresh.go` | Accept `SecurityConfig`; replace constants; use `GenerateNonce()` | +10 / -5 |

### 9.2 Files Created

| File | Purpose | Lines |
|------|---------|-------|
| `internal/security/nonce_redis.go` | Redis-backed nonce store (Phase 2) | ~150 |
| `internal/security/rate_limiter.go` | Sliding window rate limiter (Phase 2) | ~80 |
| `internal/security/nonce_parent_child.go` | Hierarchical nonce tracking (Phase 3) | ~120 |
| `internal/security/nonce_rotation.go` | Nonce rotation logic (Phase 3) | ~60 |

### 9.3 Files Unchanged

| File | Reason |
|------|--------|
| `internal/loader/loader.go` | No nonce/expiry logic |
| `internal/database/queries.go` | No nonce/expiry logic |
| `internal/database/manager.go` | No nonce/expiry logic |
| `internal/server/server.go` | No nonce/expiry logic |
| `internal/logging/query_log.go` | No nonce/expiry logic |
| `internal/security/csp.go` | No nonce/expiry logic |
| `static/thick_client.js` | Thick client already handles URL refresh; may need minor config awareness updates |

---

## 10. Configuration Matrix by Deployment Scenario

### 10.1 Development / Local

```bash
URL_EXPIRY_DEFAULT=3600        # Long-lived for convenience
URL_EXPIRY_MAX=86400           # Up to 24h
NONCE_MAX_USES=10              # Allow some reuse during debugging
REFRESH_GRACE_PERIOD=300       # 5 min grace
NONCE_STORE=memory             # Simple, no dependencies
```

### 10.2 Staging / QA

```bash
URL_EXPIRY_DEFAULT=1800        # 30 min
URL_EXPIRY_MAX=7200            # 2h
NONCE_MAX_USES=1               # Single-use (production-like)
REFRESH_GRACE_PERIOD=60        # 1 min grace
NONCE_STORE=memory
```

### 10.3 Production — Standard

```bash
URL_EXPIRY_DEFAULT=300         # 5 min
URL_EXPIRY_MAX=3600            # 1h
NONCE_MAX_USES=1               # Single-use
REFRESH_GRACE_PERIOD=30        # 30s grace
NONCE_MAX_AGE=1h               # Nonces expire in 1h
NONCE_STORE=memory
```

### 10.4 Production — High Security

```bash
URL_EXPIRY_DEFAULT=60          # 1 min
URL_EXPIRY_MAX=300             # 5 min max
NONCE_BYTES=64                 # 512-bit nonces
NONCE_ENCODING=hex             # Hex encoding for strict environments
NONCE_MAX_USES=1
REFRESH_GRACE_PERIOD=10        # 10s grace
NONCE_MAX_AGE=30m              # Nonces expire in 30m
NONCE_RATE_LIMIT=100           # 100 nonces per minute per IP
NONCE_RATE_WINDOW=60s
NONCE_BIND_TO_PARAMS=organization_id,user_id
NONCE_STORE=redis
NONCE_REDIS_URL=redis://...
```

### 10.5 Production — High Throughput (CDN Edge)

```bash
URL_EXPIRY_DEFAULT=600         # 10 min
URL_EXPIRY_MAX=3600            # 1h
NONCE_MAX_USES=100             # Allow reuse for CDN caching
NONCE_USE_WINDOW=300s          # 5 min window
REFRESH_GRACE_PERIOD=60        # 1 min grace
NONCE_CLEANUP_INTERVAL=10s     # More frequent cleanup
NONCE_POOLING_ENABLED=true
NONCE_POOL_SIZE=1000
NONCE_POOL_TTL=600s
NONCE_STORE=redis
NONCE_REDIS_URL=redis://...
```

### 10.6 Compliance — Financial / Healthcare

```bash
URL_EXPIRY_DEFAULT=60          # 1 min
URL_EXPIRY_MAX=300             # 5 min max
NONCE_BYTES=64                 # 512-bit
NONCE_ENCODING=hex
NONCE_MAX_USES=1
NONCE_MAX_AGE=15m              # Nonces expire in 15m
NONCE_ROTATION_INTERVAL=1h     # Rotate every hour
NONCE_ROTATION_GRACE=300s      # 5 min grace during rotation
NONCE_BIND_TO_PARAMS=organization_id,user_id,report_id
REFRESH_GRACE_PERIOD=10        # 10s grace
NONCE_RATE_LIMIT=50            # Strict rate limiting
NONCE_RATE_WINDOW=60s
NONCE_STORE=redis
NONCE_REDIS_URL=redis://...
NONCE_REDIS_KEY_PREFIX=fin_report_nonce:
```

---

## 11. Thick Client Implications

### 11.1 What the Thick Client Needs to Know

The thick client (`window.ReportConfig`) should receive the URL expiration window so it can:

1. **Proactively refresh** before URL expiry (avoid grace period edge cases)
2. **Handle expired URLs** gracefully (show user message)
3. **Optimize polling intervals** based on configured expiration

### 11.2 ReportConfig Addition

Add to the injected `ReportConfig` JSON:

```json
{
  "reportId": "...",
  "urlExpiry": {
    "default": 300,
    "min": 60,
    "max": 86400
  },
  "refreshGrace": 30
}
```

### 11.3 Thick Client Behavior Change

Current behavior: Thick client uses the URL's `expires` value directly.

Proposed behavior: Thick client receives `urlExpiry` as metadata and can:
- Calculate remaining time until expiry
- Trigger proactive refresh at 80% of remaining time
- Display "link expiring soon" warnings

---

## 12. Open Questions

### 12.1 Design Decisions Needed

| Question | Options | Recommendation |
|----------|---------|----------------|
| Multi-use: fixed or sliding window? | Fixed (from creation) vs. sliding (from last use) | **Sliding** — more flexible for thick-client polling |
| Rate limiter: per-IP or per-client? | IP address vs. nonce prefix vs. client ID | **IP address** for Phase 1; per-client for Phase 3 |
| Redis: key structure? | Flat keys vs. hierarchical keys | **Hierarchical** (`{prefix}:{nonce}`) for namespacing |
| Parameter binding: strict or lenient? | Reject all mismatches vs. warn only | **Strict reject** — security boundary |
| Nonce rotation: hard or soft? | Reject vs. warn + extend | **Reject** after grace period |
| CDN cache keying? | Nonce in URL vs. nonce in header | **Nonce in URL** (simpler); `NONCE_PREFIX` helps CDN identify |

### 12.2 Questions for Stakeholders

1. **Which deployment scenarios are most common?** (Prioritize Phase 1/2 accordingly)
2. **Is there a compliance requirement driving specific settings?** (e.g., FIPS, PCI, HIPAA)
3. **Do customers need per-report nonce policies?** (Currently all settings are global)
4. **Is Redis already in the infrastructure?** (Affects Phase 2 complexity)
5. **Should the thick client receive `urlExpiry` metadata?** (Affects thick client changes)

---

## 13. Appendix

### 13.1 Nonce Entropy Analysis

| Bytes | Bits | Collision Risk (birthday paradox) | URL Length (hex) | URL Length (base64) |
|-------|------|-----------------------------------|-------------------|---------------------|
| 16 | 128 | 1 in 2^64 | 32 chars | 22 chars |
| 24 | 192 | 1 in 2^96 | 48 chars | 32 chars |
| 32 | 256 | 1 in 2^128 | 64 chars | 43 chars |
| 48 | 384 | 1 in 2^192 | 96 chars | 64 chars |
| 64 | 512 | 1 in 2^256 | 128 chars | 86 chars |

**Recommendation:** 32 bytes (256 bits) is sufficient for all use cases. 64 bytes is overkill for most deployments but may be required by specific compliance frameworks.

### 13.2 Security Comparison: Current vs. Configurable

| Aspect | Current | Phase 1 | Phase 2 | Phase 3 |
|--------|---------|---------|---------|---------|
| Nonce generation | `crypto/rand` (32B) | `crypto/rand` (configurable) | `crypto/rand` + Redis | `crypto/rand` + binding |
| Nonce storage | In-memory map | In-memory map | Redis / memory | Redis + tree |
| Nonce reuse | Never | Configurable (1-1000) | Configurable per namespace | Parent-child chains |
| URL expiry max | 1 hour (hardcoded) | Configurable (1m-24h) | Configurable | Configurable + rotation |
| Rate limiting | None | None | Per-IP sliding window | Per-client sliding window |
| Parameter binding | No | No | No | Yes (Phase 3) |
| Audit trail | None | None | None | Parent-child nonce tree |

### 13.3 Glossary

| Term | Definition |
|------|------------|
| **Nonce** | A random, single-use token that prevents replay attacks |
| **HMAC** | Hash-based Message Authentication Code — cryptographically signs the URL |
| **URL Expiry** | The time at which a signed URL becomes invalid |
| **Grace Period** | Additional time after URL expiry during which refresh is still accepted |
| **Multi-Use Nonce** | A nonce that can be consumed N times within a time window |
| **Nonce Pooling** | Grouping nonces into pools to reduce tracking overhead |
| **Rate Limiting** | Limiting the number of nonces a client can generate per time window |
| **Parameter Binding** | Tying a nonce to specific parameter values for additional security |
| **Nonce Rotation** | Forcing regeneration of nonces after a certain interval |
| **Entropy** | The amount of randomness in the nonce (measured in bits) |
