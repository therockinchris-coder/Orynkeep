"""CI-only HTTP smoke test; requires an EMPTY disposable installation."""
import json
import os
import urllib.error
import urllib.request

token = os.environ["SETUP_TOKEN"]
base = "http://127.0.0.1:8088"


def request(path, body=None, supplied_token=None):
    headers = {}
    if body is not None:
        headers["Content-Type"] = "application/json"
    if supplied_token is not None:
        headers["X-Setup-Token"] = supplied_token
    req = urllib.request.Request(
        base + path,
        data=None if body is None else json.dumps(body).encode(),
        headers=headers,
    )
    try:
        response = urllib.request.urlopen(req, timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.code, json.load(response)


code, status = request("/api/setup/status")
assert code == 200 and status == {"complete": False, "enabled": True}
body = {"email": "ci-admin@example.com", "password": "CI-only-long-passphrase"}
assert request("/api/setup", body, "wrong-token")[0] == 403
assert request("/api/setup", body, token) == (201, {"complete": True})
assert request("/api/setup/status") == (200, {"complete": True, "enabled": False})
assert request("/api/setup", body, token)[0] == 409
print("Setup HTTP smoke checks passed")
