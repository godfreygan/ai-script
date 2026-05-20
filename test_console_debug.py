"""Debug console errors per page"""
import time
from playwright.sync_api import sync_playwright

BASE = "http://localhost:5173"
ADMIN = {"username": "admin", "password": "123456"}

def login(page):
    page.goto(f"{BASE}/login", wait_until="networkidle", timeout=15000)
    page.fill('input[id="username"]', ADMIN["username"])
    page.fill('input[id="password"]', ADMIN["password"])
    page.click('button[type="submit"]')
    page.wait_for_url(f"{BASE}/", timeout=10000)

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)

    pages = [
        ("/models", "models"),
        ("/users", "users"),
        ("/projects", "projects"),
        ("/scripts", "scripts"),
        ("/pipelines", "pipelines"),
        ("/reviews", "reviews"),
        ("/billing", "billing"),
        ("/publish", "publish"),
    ]

    for path, name in pages:
        context = browser.new_context(viewport={"width": 1440, "height": 900})
        page = context.new_page()
        errors = []
        def on_console(msg):
            if msg.type == "error" and "useForm" in msg.text:
                errors.append(msg.text)
        page.on("console", on_console)

        login(page)
        page.goto(f"{BASE}{path}", wait_until="networkidle", timeout=15000)
        time.sleep(1)

        # Click primary buttons
        btns = page.query_selector_all('button.ant-btn-primary')
        for btn in btns[:2]:
            try:
                btn.click()
                time.sleep(0.5)
            except:
                pass

        # Close modals/drawers
        for sel in ['.ant-modal-close', '.ant-drawer-close-button']:
            try:
                el = page.query_selector(sel)
                if el: el.click()
            except:
                pass

        if errors:
            print(f"[{name}] useForm warnings: {len(errors)}")
            for e in errors:
                print(f"  - {e[:150]}")
        else:
            print(f"[{name}] OK - no useForm warnings")

        context.close()

    browser.close()
