"""Assert external/internal balancing and DNS discovery of a new Compose service."""

import json
from pathlib import Path
import re
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

from smoke import BASE, read_env, request


def compose(*args, **kwargs):
    return subprocess.run(["docker", "compose", *args], check=True, capture_output=True, text=True, **kwargs).stdout.strip()


def internal_instances(host, allow_failure=False):
    found = set()
    for _ in range(12):
        output = subprocess.run(
            ["docker", "compose", "exec", "-T", "reports-1", "wget", "-S", "-q", "-O", "/dev/null", "http://" + host + "/healthz"],
            capture_output=True, text=True,
        )
        if output.returncode != 0:
            assert allow_failure, "internal HTTP request failed"
            continue
        match = re.search(r"X-Service-Instance:\s*(\S+)", output.stderr, re.IGNORECASE)
        assert match, "instance response header missing"
        found.add(match.group(1))
    return found


def main():
    credentials = read_env("employees/.env")
    body, _ = request(BASE + "/api/v1/auth/login", data={"username": credentials["AUTH_USERNAME"], "password": credentials["AUTH_PASSWORD"]})
    token = json.loads(body)["access_token"]
    for host in ("employees-service", "reports-service", "files-service"):
        assert len(internal_instances(host)) >= 2, host + " is not balanced internally"
    employees = internal_instances("employees-service")
    external = set()
    for _ in range(12):
        _, headers = request(BASE + "/api/v1/employees", token=token)
        external.add(headers["X-Service-Instance"])
    assert len(external) >= 2, "public gateway is not balancing"
    for host in ("employees-service", "reports-service", "files-service"):
        req = urllib.request.Request("http://localhost:8080/api/v1/employees", headers={"Host": host})
        try:
            with urllib.request.urlopen(req, timeout=10) as response:
                raise AssertionError("internal route exposed publicly: " + str(response.status))
        except urllib.error.HTTPError as error:
            assert error.code == 404
    gateway = compose("ps", "-q", "nginx")
    started = subprocess.check_output(["docker", "inspect", "--format", "{{.State.StartedAt}}", gateway], text=True)
    image = subprocess.check_output(["docker", "inspect", "--format", "{{.Image}}", compose("ps", "-q", "employees-1")], text=True).strip()
    root = Path(__file__).resolve().parent.parent
    with tempfile.NamedTemporaryFile(mode="w", suffix=".yml") as override:
        json.dump({"services": {"employees-probe": {
            "extends": {"file": str(root / "docker-compose.yml"), "service": "employees-1"}, "image": image,
        }}}, override)
        override.flush()
        cmd = ["-f", str(root / "docker-compose.yml"), "-f", override.name]
        try:
            compose(*cmd, "up", "-d", "--no-deps", "--no-build", "employees-probe")
            instance = compose(*cmd, "exec", "-T", "employees-probe", "hostname")
            for _ in range(30):
                if instance in internal_instances("employees-service", allow_failure=True):
                    break
                time.sleep(.5)
            else:
                raise AssertionError("new Compose service was not discovered")
            assert subprocess.check_output(["docker", "inspect", "--format", "{{.State.StartedAt}}", gateway], text=True) == started
            print("OK: public/internal requests are balanced; new Compose service discovered without Nginx restart", flush=True)
        finally:
            compose(*cmd, "rm", "-s", "-f", "employees-probe")
    for _ in range(30):
        if internal_instances("employees-service", allow_failure=True) == employees:
            break
        time.sleep(.5)
    else:
        raise AssertionError("removed instance remains in upstream")
    try:
        compose("stop", "employees-2")
        for _ in range(30):
            remaining = internal_instances("employees-service", allow_failure=True)
            if remaining and len(remaining) == 1:
                break
            time.sleep(.5)
        else:
            raise AssertionError("stopped replica remains in upstream")
        for _ in range(12):
            request(BASE + "/api/v1/employees", token=token)
    finally:
        compose("start", "employees-2")
    print("OK: removed/stopped instances leave upstream; remaining replica serves requests; internal routes remain private", flush=True)


if __name__ == "__main__":
    main()
