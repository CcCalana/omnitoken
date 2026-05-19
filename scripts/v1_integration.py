#!/usr/bin/env python3
"""
V1 integration smoke for T-INT.

Safety:
- never prints provider keys, virtual keys, Authorization headers, or prompts
- upstream chat is skipped unless OMNITOKEN_RUN_REAL_UPSTREAM=1
"""
import json
import os
import sys
import time
import urllib.error
import urllib.request

GATEWAY = os.environ.get("GATEWAY_URL", "http://localhost:8080").rstrip("/")
ADMIN = os.environ.get("ADMIN_URL", "http://localhost:8081").rstrip("/")
ADMIN_EMAIL = os.environ.get("OMNITOKEN_E2E_ADMIN_EMAIL", "admin@democorp.local")
ADMIN_PASSWORD = os.environ.get("OMNITOKEN_E2E_ADMIN_PASSWORD", "password")
VIEWER_EMAIL = os.environ.get("OMNITOKEN_E2E_VIEWER_EMAIL", "user01@democorp.local")
VIEWER_PASSWORD = os.environ.get("OMNITOKEN_E2E_VIEWER_PASSWORD", "password")
ORG_ID = os.environ.get("OMNITOKEN_E2E_ORG_ID", "00000000-0000-0000-0000-000000000001")
ADMIN_USER_ID = os.environ.get("OMNITOKEN_E2E_ADMIN_USER_ID", "00000000-0000-0000-0000-000000000201")
RUN_REAL_UPSTREAM = os.environ.get("OMNITOKEN_RUN_REAL_UPSTREAM", "") == "1"

results = []


def log(name, status, detail=""):
    print(f"{status:5} {name}" + (f" - {detail}" if detail else ""))
    results.append((name, status, detail))


def request(method, url, token="", body=None, timeout=45):
    headers = {"Accept": "application/json"}
    data = None
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body).encode("utf-8")

    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    started = time.monotonic()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8", errors="replace")
            return resp.status, parse_json(raw), int((time.monotonic() - started) * 1000)
    except urllib.error.HTTPError as err:
        raw = err.read().decode("utf-8", errors="replace")
        return err.code, parse_json(raw), int((time.monotonic() - started) * 1000)
    except Exception as err:
        return 0, {"error": {"message": str(err), "code": "request_failed"}}, int((time.monotonic() - started) * 1000)


def parse_json(raw):
    if raw == "":
        return {}
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return {"raw": raw[:200]}


def login(email, password, want_role):
    code, body, ms = request("POST", f"{ADMIN}/api/admin/login", body={"email": email, "password": password})
    if code != 200 or "token" not in body:
        log(f"login {want_role}", "FAIL", f"status={code} code={body.get('error', {}).get('code')}")
        return ""
    token = body["token"]
    code, me, _ = request("GET", f"{ADMIN}/api/admin/me", token=token)
    role = me.get("role")
    if code == 200 and role == want_role:
        log(f"login {want_role}", "PASS", f"{ms}ms")
    else:
        log(f"login {want_role}", "FAIL", f"me_status={code} role={role}")
    return token


def require_ok(name, code, body, expected=200):
    if code == expected:
        log(name, "PASS")
        return True
    log(name, "FAIL", f"status={code} code={body.get('error', {}).get('code')}")
    return False


def chat(token, stream=False, expect_status=200):
    body = {
        "model": "chat-fast",
        "messages": [{"role": "user", "content": "Return exactly: ok"}],
        "stream": stream,
        "max_tokens": 16,
    }
    code, resp, ms = request("POST", f"{GATEWAY}/v1/chat/completions", token=token, body=body, timeout=90)
    label = "stream chat-fast" if stream else "non-stream chat-fast"
    if code == expect_status:
        detail = f"{ms}ms"
        if code == 200:
            detail += f" model={resp.get('model', 'unknown')}"
        log(label, "PASS", detail)
    else:
        log(label, "FAIL", f"status={code} code={resp.get('error', {}).get('code')}")
    return code, resp


def main():
    print("OmniToken V1 Integration Smoke")
    print(f"Admin:   {ADMIN}")
    print(f"Gateway: {GATEWAY}")
    print(f"Upstream chat: {'enabled' if RUN_REAL_UPSTREAM else 'skipped'}")

    for name, url in (("admin health", f"{ADMIN}/healthz"), ("gateway health", f"{GATEWAY}/healthz")):
        code, body, _ = request("GET", url)
        require_ok(name, code, body)

    admin_token = login(ADMIN_EMAIL, ADMIN_PASSWORD, "admin")
    viewer_token = login(VIEWER_EMAIL, VIEWER_PASSWORD, "viewer")
    if not admin_token or not viewer_token:
        return 1

    for name, path in (
        ("overview", "/api/admin/overview"),
        ("users", "/api/admin/users"),
        ("models", "/api/admin/models"),
        ("virtual models", "/api/admin/virtual-models"),
        ("audit logs", "/api/admin/audit-logs?limit=20"),
    ):
        code, body, _ = request("GET", f"{ADMIN}{path}", token=admin_token)
        require_ok(name, code, body)

    code, body, _ = request(
        "PATCH",
        f"{ADMIN}/api/admin/users/{ADMIN_USER_ID}/quota",
        token=viewer_token,
        body={"budget_cents": 1},
    )
    require_ok("viewer quota PATCH denied", code, body, expected=403)

    code, key_body, _ = request(
        "POST",
        f"{ADMIN}/api/admin/dev/virtual-keys",
        token=admin_token,
        body={"organization_id": ORG_ID, "user_id": ADMIN_USER_ID},
    )
    if not require_ok("admin creates virtual key", code, key_body, expected=201):
        return 1
    virtual_key = key_body.get("virtual_key", "")
    if not virtual_key:
        log("virtual key returned", "FAIL")
        return 1
    log("virtual key returned", "PASS", f"len={len(virtual_key)} prefix={key_body.get('key_prefix', '?')}")

    if RUN_REAL_UPSTREAM:
        request("PATCH", f"{ADMIN}/api/admin/users/{ADMIN_USER_ID}/quota", token=admin_token, body={"budget_cents": None})
        chat(virtual_key, stream=False, expect_status=200)
        chat(virtual_key, stream=True, expect_status=200)
        time.sleep(2)
        code, users, _ = request("GET", f"{ADMIN}/api/admin/users", token=admin_token)
        used = 0
        for user in users.get("users", []):
            if user.get("user_id") == ADMIN_USER_ID:
                used = int(user.get("used_budget_cents") or 0)
                break
        log("usage visible in users", "PASS" if used > 0 else "WARN", f"used_budget_cents={used}")
        request("PATCH", f"{ADMIN}/api/admin/users/{ADMIN_USER_ID}/quota", token=admin_token, body={"budget_cents": 0})
        chat(virtual_key, stream=False, expect_status=402)
    else:
        log("real Ark chat", "SKIP", "set OMNITOKEN_RUN_REAL_UPSTREAM=1 to spend tokens")

    time.sleep(1)
    code, audit, _ = request("GET", f"{ADMIN}/api/admin/audit-logs?limit=50", token=admin_token)
    actions = {item.get("action") for item in audit} if isinstance(audit, list) else set()
    required = {"create_virtual_key", "update_quota"}
    if code == 200 and required.issubset(actions):
        log("audit contains integration writes", "PASS", ",".join(sorted(actions)))
    else:
        log("audit contains integration writes", "FAIL", f"status={code} actions={sorted(actions)}")

    failed = [item for item in results if item[1] == "FAIL"]
    print(f"\nSummary: {len(results) - len(failed)}/{len(results)} non-failing checks")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
