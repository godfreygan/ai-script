import time
from playwright.sync_api import sync_playwright

BASE = "http://localhost:5173"
ADMIN = {"username": "admin", "password": "123456"}

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page(viewport={"width": 1440, "height": 900})

    page.goto(f"{BASE}/login", wait_until="networkidle", timeout=15000)
    page.fill('input[id="username"]', ADMIN["username"])
    page.fill('input[id="password"]', ADMIN["password"])
    page.click('button[type="submit"]')
    page.wait_for_url(f"{BASE}/", timeout=10000)

    page.goto(f"{BASE}/projects", wait_until="networkidle", timeout=15000)
    time.sleep(2)

    # Check table content without search
    rows = page.query_selector_all('tbody tr')
    print(f"Rows (no search): {len(rows)}")
    for i, tr in enumerate(rows[:3]):
        text = tr.inner_text().strip()[:80]
        print(f"  Row {i}: {text}")

    # Now do search like the test
    search = page.query_selector('.ant-input-search input')
    if search:
        search.fill("test")
        search.press("Enter")
        time.sleep(2)
        rows_after = page.query_selector_all('tbody tr')
        print(f"Rows (after search 'test'): {len(rows_after)}")
        for i, tr in enumerate(rows_after[:3]):
            text = tr.inner_text().strip()[:80]
            print(f"  Row {i}: {text}")

    # Check member buttons
    btns = page.query_selector_all('button')
    for btn in btns:
        text = btn.inner_text().strip()
        if text:
            print(f"Button: '{text}'")

    browser.close()
