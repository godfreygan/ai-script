#!/usr/bin/env bash
# ============================================================================
# smoke.sh - AI-Script backend E2E smoke test
# ----------------------------------------------------------------------------
# Purpose : verify backend endpoint reachability + auth chain (NOT business
#           logic). Empty lists count as success.
#
# Usage   : BASE_URL=http://localhost:8080 ./scripts/smoke.sh
#           (BASE_URL defaults to http://localhost:8080 when not set)
#
# Deps    : bash (POSIX), curl, jq  (Windows: run under git-bash)
#
# Auth    : logs in via /api/v1/auth/login with admin/admin123
#           (seeded by Agent 10's default account)
#
# Exit    : 0 on full pass, non-zero on first failure
# ============================================================================

set -u
set -o pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123}"

# ---- color helpers ---------------------------------------------------------
if [ -t 1 ]; then
    RED=$'\033[31m'
    GREEN=$'\033[32m'
    YELLOW=$'\033[33m'
    RESET=$'\033[0m'
else
    RED=""; GREEN=""; YELLOW=""; RESET=""
fi

red()    { printf '%s%s%s\n' "$RED"    "$*" "$RESET" >&2; }
green()  { printf '%s%s%s\n' "$GREEN"  "$*" "$RESET"; }
yellow() { printf '%s%s%s\n' "$YELLOW" "$*" "$RESET"; }

die() {
    red "FAIL: $*"
    exit 1
}

# ---- dep check -------------------------------------------------------------
command -v curl >/dev/null 2>&1 || die "curl not found in PATH"
command -v jq   >/dev/null 2>&1 || die "jq not found in PATH"

yellow "[smoke] BASE_URL = $BASE_URL"

# ---- 1) login -> token -----------------------------------------------------
yellow "[smoke] logging in as $USERNAME ..."

LOGIN_BODY="$(printf '{"username":"%s","password":"%s"}' "$USERNAME" "$PASSWORD")"

LOGIN_RESP="$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "$LOGIN_BODY")" || die "curl login crashed"

if [ -z "$LOGIN_RESP" ]; then
    die "login returned empty body (is the server up?)"
fi

TOKEN="$(printf '%s' "$LOGIN_RESP" | jq -r '.data.access_token // empty')"

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    red "login response: $LOGIN_RESP"
    die "could not extract .data.access_token from login response"
fi

export TOKEN
green "[smoke] login OK, token len=${#TOKEN}"

# ---- 2) GET probe ----------------------------------------------------------
# hit_get <path> -- send GET with bearer token, assert HTTP 200
hit_get() {
    local path="$1"
    local url="${BASE_URL}${path}"
    local code

    code="$(curl -s -o /dev/null -w '%{http_code}' \
        -H "Authorization: Bearer $TOKEN" \
        -H 'Accept: application/json' \
        "$url")" || { red "curl crashed for $path"; return 1; }

    if [ "$code" != "200" ]; then
        red "GET $path -> HTTP $code (expected 200)"
        return 1
    fi

    green "GET $path -> 200"
    return 0
}

# Endpoints to probe (reachability + auth only). Grouped per spec.
ENDPOINTS=(
    # current user
    "/api/v1/users/me"

    # users / depts / roles / projects / models
    "/api/v1/users"
    "/api/v1/depts"
    "/api/v1/roles"
    "/api/v1/projects"
    "/api/v1/models"

    # scripts / episodes (no flat /episodes list -> use /scripts only)
    "/api/v1/scripts"

    # storyboards / styles / images / short_videos
    # (no flat /storyboards list endpoint -> covered via styles/images/short_videos)
    "/api/v1/styles"
    "/api/v1/images"
    "/api/v1/short_videos"

    # full_videos / pipelines
    "/api/v1/full_videos"
    "/api/v1/pipelines"

    # review/flows / publishes / billing/quotas / audit_logs / feature_flags
    "/api/v1/review/flows"
    "/api/v1/publishes"
    "/api/v1/billing/quotas"
    "/api/v1/audit_logs"
    "/api/v1/feature_flags"
)

FAILED=0
TOTAL=0

for ep in "${ENDPOINTS[@]}"; do
    TOTAL=$((TOTAL + 1))
    if ! hit_get "$ep"; then
        FAILED=$((FAILED + 1))
    fi
done

# ---- 3) summary ------------------------------------------------------------
echo
if [ "$FAILED" -ne 0 ]; then
    red "SMOKE FAILED: $FAILED/$TOTAL endpoint(s) did not return 200"
    exit 1
fi

green "ALL PASSED ($TOTAL/$TOTAL endpoints reachable, auth chain OK)"
exit 0
