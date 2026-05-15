# Trust Model for Refresh Requests

The server uses **HMAC-based URL signing** to verify that a refresh request originates from a trusted session. There is no session, cookie, or token-based session model.

## How Trust Is Established

### 1. Initial Embed (Trusted Entry Point)

When a client first loads a report, it requests:

```
GET /api/embed?report_id=X&expires=Y&nonce=Z&sig=W&...
```

The embed handler validates:

1. **Expiration** — `expires < now` → rejected
2. **Nonce** — checked against `NonceTracker` (in‑memory map, configurable cleanup interval) — replay protection
3. **HMAC Signature** — the critical trust step (see below)

The server responds by rendering `reports/{reportID}/report.html` with a `<script>window.ReportConfig = {...}</script>` injected before `</body>`. The config contains:

- `currentUrl` — the full signed embed URL
- `params`, `immutableParams`, `mutableParams`, `datasources`

### 2. The HMAC Signature (Core Trust Mechanism)

**File:** `internal/security/hmac.go`

The signature is computed as:

```
message = reportID:expires:nonce:canonical_immutable_params
sig = HMAC-SHA256(message, HMAC_SECRET)
```

Where `canonical_immutable_params` is:

- Only **immutable** parameters (those listed in `report.yaml` under `immutable_params`)
- Keys sorted alphabetically, values URL-encoded, joined with `&`
- Example: `date=2024-01-01&region=us%20west`

**Key insight:** Mutable parameters are **excluded** from the HMAC. This allows the client to change mutable parameters (e.g., filter values) without invalidating the signature.

### 3. The Refresh Request (POST `/refresh`)

When the embedded client needs fresh data, it calls `POST /refresh` with:

- **URL query parameters** — the original signed params (`report_id`, `expires`, `nonce`, `sig`)
- **Request body** — JSON with `{"params": {"mutable_key": "new_value"}}`

The **refresh handler** validates trust in this exact order:

```
Step 1: Parse signed params from URL query
Step 2: Load report definition (to know which params are immutable)
Step 3: Extract original params from URL
Step 4: Merge with new params from request body
Step 5: Validate merged params against report definition
Step 6: Extract immutable params from original params
Step 7: VERIFY HMAC — recompute HMAC and compare with sig
Step 8: Check expiration with grace period (5 min)
Step 9: Execute queries → return data + new signed URL
```

**Step 7 is the trust gate.** The handler calls:

```go
immutableParams := report.ExtractImmutable(originalParams)
security.VerifyURL(reportID, expires, nonce, immutableParams, sig, h.hmacSecret)
```

`VerifyURL` recomputes `HMAC-SHA256(reportID:expires:nonce:canonical_immutable_params, HMAC_SECRET)` and uses `hmac.Equal` (constant-time comparison) against the provided `sig`.

If the signature doesn't match → **403 Forbidden "Invalid signature"**. This means either:

- The `HMAC_SECRET` is wrong
- The `report_id`, `expires`, `nonce`, or any **immutable** parameter was tampered with
- The request didn't originate from a properly signed URL

### 4. What the HMAC Protects

| Protected | How |
|-----------|-----|
| `report_id` | Included in HMAC message |
| `expires` | Included in HMAC message |
| `nonce` | Included in HMAC message |
| Immutable params (e.g., `date`, `region`) | Included in canonical params |
| Report definition | Implied — only valid immutable param names/values produce a valid sig |

| NOT Protected | Why |
|---------------|-----|
| Mutable params (e.g., `filter`, `sort`) | Intentionally excluded so client can change them |
| Request body params | Only validated against the report schema, not signed |

### 5. The "Next URL" Rotation

After a successful refresh, the handler generates a **brand new signed URL** with:

- Fresh nonce
- New `expires` (5 min from now)
- New HMAC signature over the merged params

This is returned as `next_url` in the JSON response. The client uses this for subsequent refreshes. The chain is: **each refresh produces a new signed URL for the next refresh** — there's no persistent session state on the server.

### 6. The Public Paths Bypass

If `ENABLE_PUBLIC_PATHS=true`, **all HMAC verification is skipped**. The server only validates that the report exists and params match the schema. This is explicitly a development/testing mode.

### 7. Datasource-Level Database Validation

Each datasource can optionally specify its own database via `datasources.{name}.database`. The server validates this at startup:

- At least one of `report.database` or a datasource-level `database` must be set for the report to be valid
- Database names must exist in `databases.yaml` — validation errors are logged at startup but the report still loads (runtime will fail with a clear error if the DB doesn't exist)
- This validation is a **startup-time warning only** — it does not block report loading, ensuring zero breaking changes for existing reports

### 8. Configurable Security Parameters (Phase 1)

All security timeouts and limits are now configurable via environment variables:

#### URL Expiration
- `URL_EXPIRY_DEFAULT` — default URL lifetime (default: `5m`)
- `URL_EXPIRY_MIN` — minimum allowed expiration (default: `1m`)
- `URL_EXPIRY_MAX` — maximum allowed expiration (default: `24h`)
- `REFRESH_GRACE_PERIOD` — grace period after expiry (default: `0s`)

#### NONCE Settings
- `NONCE_BYTES` — random bytes (16‑64, default: `32`)
- `NONCE_ENCODING` — encoding format (`urlsafe-base64`, `base64`, `hex`, default: `urlsafe-base64`)
- `NONCE_MAX_AGE` — maximum nonce lifetime (default: `24h`)
- `NONCE_CLEANUP_INTERVAL` — cleanup frequency (default: `60s`)
- `NONCE_MAX_USES` — max uses per nonce (default: `1`, single‑use)
- `NONCE_USE_WINDOW` — sliding window for multi‑use (default: `5m`)

**Defaults match previous hardcoded values** — zero‑downtime migration.

#### Nonce Usage Pattern
- `/api/embed` — consumes one nonce use (replay protection)
- `/refresh` — does **not** consume nonce uses (allows multiple refreshes)

See `NONCE_AND_URL_EXPIRY_CONFIG.md` for complete documentation.

## Summary

The server trusts a refresh request if and only if:

1. The **HMAC signature** on the original embed URL is valid (proving the request chain started from a properly signed URL)
2. The **nonce** hasn't been used before (prevents replay of the embed URL itself)
3. The **immutable parameters** haven't been altered
4. The **expiration** hasn't passed (with configurable grace period)

The trust is **cryptographic**, not session‑based. The `HMAC_SECRET` environment variable is the single shared secret between the URL generator (client‑side or admin tool) and the server.
