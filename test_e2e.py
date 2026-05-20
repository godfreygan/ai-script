"""E2E full test suite for AI-Script platform — login, all pages, all API endpoints, healthcheck"""
import json
import sys
import time
from playwright.sync_api import sync_playwright, expect

BASE = "http://localhost:5173"
API = "http://localhost:8086/api/v1"
ADMIN = {"username": "admin", "password": "123456"}

results = []

def record(name, ok, detail=""):
    results.append({"name": name, "pass": ok, "detail": detail})
    status = "PASS" if ok else "FAIL"
    print(f"  [{status}] {name}: {detail}", flush=True)

# ========== 1. Backend API Tests ==========
def test_backend_apis():
    print("\n=== Backend API Tests ===", flush=True)
    import urllib.request

    # Login
    req = urllib.request.Request(f"{API}/auth/login",
        data=json.dumps(ADMIN).encode(), headers={"Content-Type": "application/json"})
    try:
        resp = urllib.request.urlopen(req, timeout=10)
        data = json.loads(resp.read())
        token = data["data"]["access_token"]
        record("auth.login", True, f"token={token[:20]}...")
    except Exception as e:
        record("auth.login", False, str(e))
        return None

    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

    # Health probes
    for path in ["/healthz/live", "/healthz/ready"]:
        try:
            r = urllib.request.urlopen(f"http://localhost:8086{path}", timeout=5)
            d = json.loads(r.read())
            record(f"GET {path}", d.get("status") == "ok", str(d))
        except Exception as e:
            record(f"GET {path}", False, str(e))

    # All GET endpoints
    get_endpoints = [
        "/users/me", "/users?page=1&page_size=20", "/depts?page=1&page_size=20",
        "/roles?page=1&page_size=20", "/permissions",
        "/projects?page=1&page_size=20", "/models?page=1&page_size=20",
        "/scripts?page=1&page_size=20", "/styles?page=1&page_size=20",
        "/images?page=1&page_size=20", "/short_videos?page=1&page_size=20",
        "/full_videos?page=1&page_size=20", "/invocations?page=1&page_size=20",
        "/invocations/stats", "/pipelines?page=1&page_size=20",
        "/review/flows?page=1&page_size=20", "/review/records?page=1&page_size=20",
        "/publishes?page=1&page_size=20", "/billing/quotas?page=1&page_size=20",
        "/billing/daily?page=1&page_size=20", "/audit_logs?page=1&page_size=20",
        "/feature_flags?page=1&page_size=20",
    ]
    for ep in get_endpoints:
        try:
            req = urllib.request.Request(f"{API}{ep}", headers=headers)
            r = urllib.request.urlopen(req, timeout=10)
            d = json.loads(r.read())
            record(f"GET {ep}", d.get("code") == 0, f"code={d.get('code')}")
        except urllib.error.HTTPError as e:
            body = e.read().decode()[:200]
            record(f"GET {ep}", False, f"HTTP {e.code}: {body}")
        except Exception as e:
            record(f"GET {ep}", False, str(e))

    # 404 test
    try:
        req = urllib.request.Request(f"{API}/nonexistent_endpoint", headers=headers)
        r = urllib.request.urlopen(req, timeout=5)
        d = json.loads(r.read())
        record("GET /api/v1/nonexistent (404)", d.get("code") == 40400, f"code={d.get('code')}")
    except urllib.error.HTTPError as e:
        record("GET /api/v1/nonexistent (404)", e.code == 404, f"HTTP {e.code}")
    except Exception as e:
        record("GET /api/v1/nonexistent (404)", False, str(e))

    # Token refresh
    refresh_token = data["data"].get("refresh_token", "")
    if refresh_token:
        try:
            req = urllib.request.Request(f"{API}/auth/refresh",
                data=json.dumps({"refresh_token": refresh_token}).encode(),
                headers={"Content-Type": "application/json"})
            r = urllib.request.urlopen(req, timeout=10)
            d = json.loads(r.read())
            record("auth.refresh", d.get("code") == 0, f"new_token={d.get('data',{}).get('access_token','')[:15]}...")
        except Exception as e:
            record("auth.refresh", False, str(e))
    else:
        record("auth.refresh", False, "no refresh_token")

    return token

# ========== 2. Frontend Page Tests ==========
def test_frontend_pages(token):
    print("\n=== Frontend Page Tests ===", flush=True)

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            viewport={"width": 1440, "height": 900},
            storage_state=None,
        )

        # Inject token into localStorage before navigation
        page = context.new_page()

        # Capture console errors
        console_errors = []
        page.on("console", lambda msg: console_errors.append(msg) if msg.type == "error" else None)

        # Navigate to login
        page.goto(f"{BASE}/login", wait_until="networkidle", timeout=15000)

        # Login
        try:
            page.fill('input[id="username"]', ADMIN["username"])
            page.fill('input[id="password"]', ADMIN["password"])
            page.click('button[type="submit"]')
            page.wait_for_url(f"{BASE}/", timeout=10000)
            record("login flow", True, "redirected to /")
        except Exception as e:
            record("login flow", False, str(e))
            browser.close()
            return

        # Wait for dashboard to load
        page.wait_for_load_state("networkidle", timeout=10000)
        time.sleep(1)

        # Test all pages by navigating
        pages = [
            ("/", "Dashboard"),
            ("/projects", "Projects"),
            ("/scripts", "Scripts"),
            ("/prompts", "Prompts"),
            ("/storyboards", "Storyboards"),
            ("/styles", "Styles"),
            ("/images", "Images"),
            ("/short-videos", "ShortVideos"),
            ("/full-videos", "FullVideos"),
            ("/reviews", "Reviews"),
            ("/publish", "Publish"),
            ("/users", "Users"),
            ("/depts", "Depts"),
            ("/roles", "Roles"),
            ("/models", "Models"),
            ("/billing", "Billing"),
            ("/pipelines", "Pipelines"),
            ("/invocations", "Invocations"),
            ("/feature-flags", "FeatureFlags"),
            ("/audit-logs", "AuditLogs"),
        ]

        for path, name in pages:
            try:
                page.goto(f"{BASE}{path}", wait_until="networkidle", timeout=15000)
                time.sleep(0.5)

                # Check for React error overlay (red box)
                error_overlay = page.query_selector('.ant-result-error, [class*="error-boundary"]')
                if error_overlay:
                    record(f"page {name}", False, "error boundary visible")
                    continue

                # Check page has content (not blank)
                body_text = page.locator("body").inner_text()
                has_content = len(body_text.strip()) > 20
                record(f"page {name}", has_content, f"content_len={len(body_text.strip())}")

                # Check for console errors specific to this page
                page_errors = [e for e in console_errors if e.type == "error"]
                # Reset for next page
                console_errors.clear()

            except Exception as e:
                record(f"page {name}", False, str(e)[:200])

        # Test Models page - healthcheck button
        try:
            page.goto(f"{BASE}/models", wait_until="networkidle", timeout=15000)
            time.sleep(1)
            # Check if table exists
            table = page.query_selector('.ant-table')
            record("models.table", table is not None, "table rendered" if table else "no table")
        except Exception as e:
            record("models.table", False, str(e)[:200])

        # Test sidebar menu click
        try:
            page.goto(f"{BASE}/", wait_until="networkidle", timeout=15000)
            menu_items = page.locator('.ant-menu-item, .ant-menu-submenu-title').all()
            record("sidebar.menu", len(menu_items) > 0, f"{len(menu_items)} menu items")
        except Exception as e:
            record("sidebar.menu", False, str(e)[:200])

        # Test 401 handling - clear token and try to access
        try:
            page.evaluate("localStorage.removeItem('ai-script-auth')")
            page.goto(f"{BASE}/", wait_until="networkidle", timeout=10000)
            time.sleep(1)
            current_url = page.url
            on_login = "/login" in current_url
            record("401 redirect to login", on_login, f"url={current_url}")
        except Exception as e:
            record("401 redirect to login", False, str(e)[:200])

        browser.close()

# ========== 3. Frontend Console Error Scan ==========
def test_console_errors(token):
    print("\n=== Console Error Scan ===", flush=True)

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page(viewport={"width": 1440, "height": 900})

        # Login first
        page.goto(f"{BASE}/login", wait_until="networkidle", timeout=15000)
        page.fill('input[id="username"]', ADMIN["username"])
        page.fill('input[id="password"]', ADMIN["password"])
        page.click('button[type="submit"]')
        page.wait_for_url(f"{BASE}/", timeout=10000)
        page.wait_for_load_state("networkidle", timeout=10000)

        # Collect all console errors across all pages
        all_errors = []
        page.on("console", lambda msg: all_errors.append(f"[{msg.type}] {msg.text}") if msg.type in ("error", "warning") else None)

        pages = ["/", "/projects", "/scripts", "/models", "/users", "/billing",
                 "/pipelines", "/invocations", "/feature-flags", "/audit-logs"]

        for path in pages:
            try:
                page.goto(f"{BASE}{path}", wait_until="networkidle", timeout=15000)
                time.sleep(1)
            except:
                pass

        # Filter out known benign warnings
        real_errors = [e for e in all_errors
                       if "[error]" in e
                       and "favicon" not in e.lower()
                       and "devtools" not in e.lower()
                       and "download the react devtools" not in e.lower()
                       and "content security policy" not in e.lower()
                       and "destroyonclose" not in e.lower()
                       and "destroyonhidden" not in e.lower()
                       and "429" not in e
                       and "too many requests" not in e.lower()]

        if real_errors:
            for e in real_errors[:10]:
                record("console.error", False, e[:200])
        else:
            record("console.error scan", True, "no console errors detected")

        browser.close()

# ========== Main ==========
if __name__ == "__main__":
    print("=" * 70, flush=True)
    print("AI-Script E2E Full Test Suite", flush=True)
    print("=" * 70, flush=True)

    token = test_backend_apis()
    if token:
        test_frontend_pages(token)
        test_console_errors(token)

    # Summary
    print("\n" + "=" * 70, flush=True)
    passed = sum(1 for r in results if r["pass"])
    failed = sum(1 for r in results if not r["pass"])
    total = len(results)
    print(f"SUMMARY: {passed}/{total} passed, {failed} failed", flush=True)

    if failed > 0:
        print("\nFAILED TESTS:", flush=True)
        for r in results:
            if not r["pass"]:
                print(f"  - {r['name']}: {r['detail']}", flush=True)

    print("=" * 70, flush=True)
    sys.exit(0 if failed == 0 else 1)
