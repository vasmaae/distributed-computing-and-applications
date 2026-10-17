"""Check JWT, reports, dashboards, Kafka UI, traces and centralized JSON logs."""

import base64
import csv
import io
import json
from collections import Counter
from pathlib import Path
import time
import urllib.error
import urllib.request
import urllib.parse
import uuid


BASE = "http://localhost:8080/employees"


def request(url, *, token=None, basic_auth=None, data=None, trace_id=None, status=200):
    headers = {}
    if token:
        headers["Authorization"] = "Bearer " + token
    if basic_auth:
        credentials = base64.b64encode(":".join(basic_auth).encode()).decode()
        headers["Authorization"] = "Basic " + credentials
    if data is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(data).encode()
    if trace_id:
        headers["traceparent"] = "00-" + trace_id + "-" + uuid.uuid4().hex[:16] + "-01"
    req = urllib.request.Request(url, data=data, headers=headers)
    try:
        response = urllib.request.urlopen(req, timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        assert response.code == status, f"{url}: expected {status}, got {response.code}"
        return response.read(), response.headers


def read_env(path):
    # shortcut: plain KEY=value env files; use a dotenv parser if quoted or multiline values are needed.
    return dict(
        line.split("=", 1)
        for line in (Path(__file__).resolve().parent.parent / path).read_text().splitlines()
        if line and not line.startswith("#") and "=" in line
    )


def main():
    settings = read_env("employees/.env")
    request(BASE + "/api/v1/employees", status=401)
    request(BASE + "/api/v1/auth/login", data={"username": settings["AUTH_USERNAME"], "password": "wrong"}, status=401)
    body, _ = request(BASE + "/api/v1/auth/login", data={"username": settings["AUTH_USERNAME"], "password": settings["AUTH_PASSWORD"]})
    token = json.loads(body)["access_token"]
    request(BASE + "/api/v1/employees", token=token + ".invalid", status=401)
    instances = set()
    for _ in range(12):
        _, headers = request(BASE + "/api/v1/employees", token=token)
        instances.add(headers.get("X-Service-Instance"))
    assert None not in instances and len(instances) >= 2, "gateway does not balance employee instances"
    trace_id = uuid.uuid4().hex
    body, _ = request(BASE + "/api/v1/reports/employees-report", token=token, trace_id=trace_id)
    report = json.loads(body)
    assert report["total"] == len(report["employees"])
    assert report["is_fired"] == sum(e["is_fired"] for e in report["employees"])
    file_url = BASE + "/api/v1/reports/files/" + report["file_id"]
    for _ in range(30):
        req = urllib.request.Request(file_url, headers={"Authorization": "Bearer " + token})
        try:
            with urllib.request.urlopen(req, timeout=10) as response:
                assert response.headers["Content-Type"].startswith("text/csv")
                stored = response.read().decode()
                break
        except urllib.error.HTTPError as error:
            if error.code != 404:
                raise
            time.sleep(0.5)
    else:
        raise AssertionError("queued CSV was not saved within 15 seconds")
    rows = list(csv.DictReader(io.StringIO(stored)))
    assert len(rows) == report["total"]
    expected = {
        e["id"]: {"id": e["id"], "fio": e["fio"], "birth_date": e["birth_date"][:10], "is_fired": str(e["is_fired"]).lower()}
        for e in report["employees"]
    }
    assert {row["id"]: row for row in rows} == expected
    request(file_url, status=401)
    request(BASE + "/api/v1/reports/files/wrong", token=token, status=400)
    request(BASE + "/api/v1/reports/files/" + str(uuid.uuid4()), token=token, status=404)
    request(BASE + "/metrics", status=404)
    print("OK: JWT, queued CSV and Future download through gateway", flush=True)

    for _ in range(40):
        body, _ = request("http://localhost:9090/api/v1/targets")
        targets = json.loads(body)["data"]["activeTargets"]
        counts = Counter(t["labels"]["job"] for t in targets if t["health"] == "up")
        if all(counts[job] >= 2 for job in ("employees", "reports", "files")):
            break
        time.sleep(0.5)
    else:
        raise AssertionError("Prometheus cannot scrape two instances of each service")

    for _ in range(40):
        try:
            with urllib.request.urlopen("http://localhost:16686/api/traces/" + trace_id, timeout=10) as response:
                traces = json.load(response)["data"]
        except urllib.error.HTTPError as error:
            if error.code != 404:
                raise
            traces = []
        if traces:
            services = {p["serviceName"] for p in traces[0]["processes"].values()}
            operations = {s["operationName"] for s in traces[0]["spans"]}
            if {"employees-service", "reports-service", "files-service"} <= services and {"reports.files publish", "reports.files process"} <= operations:
                break
        time.sleep(0.5)
    else:
        raise AssertionError("trace does not connect HTTP calls and Kafka delivery")
    print("OK: Prometheus scrapes all services; Jaeger trace http://localhost:16686/trace/" + trace_id)

    body, _ = request("http://localhost:8081/api/clusters")
    clusters = json.loads(body)
    assert any(cluster["name"] == "local" for cluster in clusters), "Kafka UI cluster missing"
    body, _ = request("http://localhost:8081/api/clusters/local/topics/reports.files")
    assert json.loads(body)["name"] == "reports.files", "Kafka UI cannot read report topic"

    grafana = read_env("monitoring/.env")
    credentials = (grafana["GF_SECURITY_ADMIN_USER"], grafana["GF_SECURITY_ADMIN_PASSWORD"])
    body, _ = request("http://localhost:3000/api/dashboards/uid/services", basic_auth=credentials)
    dashboard = json.loads(body)["dashboard"]
    assert len(dashboard["panels"]) == 6, "Grafana service dashboard missing"
    for panel in dashboard["panels"]:
        expression = panel["targets"][0]["expr"].replace("$service", "employees|reports|files").replace("$__rate_interval", "1m").replace("$__range", "15m")
        body, _ = request("http://localhost:3000/api/datasources/proxy/uid/prometheus/api/v1/query?" + urllib.parse.urlencode({"query": expression}), basic_auth=credentials)
        assert json.loads(body)["status"] == "success", panel["title"]
    print("OK: Kafka UI reads reports.files; all 6 Grafana panels query Prometheus")

    for _ in range(90):
        body, _ = request("http://localhost:9200/app-logs-*/_search?ignore_unavailable=true", data={
            "query": {"term": {"trace_id": trace_id}}, "size": 100,
        })
        logs = [hit["_source"] for hit in json.loads(body)["hits"]["hits"]]
        if {"employees-service", "reports-service", "files-service"} <= {entry["service"] for entry in logs}:
            if {"published queued report", "saved queued report"} <= {entry["message"] for entry in logs}:
                break
        time.sleep(0.5)
    else:
        raise AssertionError("ELK did not index correlated HTTP and Kafka logs from all services")
    serialized = json.dumps(logs)
    for secret in (settings["AUTH_PASSWORD"], settings["JWT_SECRET"], token):
        assert secret not in serialized, "secret leaked into centralized logs"
    for entry in logs:
        assert entry["@timestamp"] and entry["level"] and entry["span_id"] and entry["instance_id"], "log fields missing"
        assert "_jsonparsefailure" not in entry.get("tags", []), "invalid application JSON log"
    body, _ = request("http://localhost:5601/api/data_views/data_view/app-logs")
    assert json.loads(body)["data_view"]["title"] == "app-logs-*", "Kibana data view missing"
    body, _ = request("http://localhost:5601/api/data_views/default")
    assert json.loads(body)["data_view_id"] == "app-logs", "Kibana default data view missing"
    print("OK: ELK stores JSON logs from all services; search trace_id: " + trace_id)


if __name__ == "__main__":
    main()
