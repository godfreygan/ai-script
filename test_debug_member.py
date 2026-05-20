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

    # Get all button texts
    btns = page.query_selector_all('button')
    print(f"Total buttons: {len(btns)}")
    for btn in btns:
        text = btn.inner_text().strip()
        if text:
            print(f"  Button: '{text}'")

    # Try different selectors
    member1 = page.query_selector_all('button:has-text("成员")')
    print(f"button:has-text('成员'): {len(member1)}")

    member2 = page.query_selector_all('text=成员')
    print(f"text=成员: {len(member2)}")

    member3 = page.query_selector_all('button >> text=成员')
    print(f"button >> text=成员: {len(member3)}")

    # Check table rows
    rows = page.query_selector_all('.ant-table-row')
    print(f"Table rows: {len(rows)}")

    browser.close()
