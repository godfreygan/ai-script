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

    # Check for ant-table-row
    rows = page.query_selector_all('.ant-table-row')
    print(f".ant-table-row count: {len(rows)}")

    # Check for any tr inside tbody
    trs = page.query_selector_all('tbody tr')
    print(f"tbody tr count: {len(trs)}")

    # Get classes of first few trs
    for i, tr in enumerate(trs[:3]):
        classes = tr.get_attribute('class') or ''
        print(f"  tr {i} classes: {classes}")

    # Check buttons
    btns = page.query_selector_all('button')
    for btn in btns:
        text = btn.inner_text().strip()
        if text:
            print(f"Button: '{text}'")

    browser.close()
