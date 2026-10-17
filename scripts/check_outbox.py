"""Check durable acceptance during a Kafka outage and recovery after process/DB restarts."""

import hashlib
import json
from pathlib import Path
import subprocess
import time
import urllib.error
import uuid

from check_instances import compose
from smoke import BASE, read_env, request


BROKERS = ("kafka-1", "kafka-2", "kafka-3")
REPORTS = ("reports-1", "reports-2")


def sql(statement):
    return compose("exec", "-T", "postgres", "sh", "-c", 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d reports -At', input=statement)


def healthy(*services):
    for _ in range(120):
        ids = compose("ps", "-q", *services).splitlines()
        if len(ids) == len(services):
            states = subprocess.check_output(["docker", "inspect", "--format", "{{.State.Health.Status}}", *ids], text=True).splitlines()
            if all(state == "healthy" for state in states):
                return
        time.sleep(.5)
    raise AssertionError("containers not healthy: " + ", ".join(services))


def saved_logs(file_id):
    body, _ = request("http://localhost:9200/app-logs-*/_count", data={"query": {"bool": {"filter": [
        {"term": {"file_id": file_id}}, {"match_phrase": {"message": "saved queued report"}},
    ]}}})
    return json.loads(body)["count"]


def main():
    settings = read_env("employees/.env")
    body, _ = request(BASE + "/api/v1/auth/login", data={"username": settings["AUTH_USERNAME"], "password": settings["AUTH_PASSWORD"]})
    token = json.loads(body)["access_token"]
    reports = []
    try:
        compose("stop", *BROKERS)
        compose("stop", "nginx")
        compose("up", "-d", "--no-build", "nginx")
        assert not set(BROKERS) & set(compose("ps", "--services", "--status", "running").splitlines()), "gateway startup unexpectedly started Kafka"
        for _ in range(40):
            try:
                request(BASE + "/api/v1/employees", token=token)
                break
            except (AssertionError, urllib.error.URLError):
                time.sleep(.5)
        else:
            raise AssertionError("gateway did not start without Kafka")
        for _ in range(2):
            started = time.monotonic()
            body, _ = request(BASE + "/api/v1/reports/employees-report", token=token)
            report = json.loads(body)
            file_id = str(uuid.UUID(report["file_id"]))
            assert time.monotonic() - started < 5, "HTTP acceptance still waits for Kafka"
            row = json.loads(sql("SELECT json_build_object('pending', published_at IS NULL, 'payload', encode(csv, 'hex')) FROM report_outbox WHERE file_id = '" + file_id + "';"))
            assert row["pending"] and row["payload"], "CSV was not retained in outbox"
            reports.append((file_id, bytes.fromhex(row["payload"])))
        compose("stop", *REPORTS)
        compose("restart", "postgres")
        healthy("postgres")
        compose("start", *REPORTS)
        healthy(*REPORTS)
        for file_id, payload in reports:
            stored = sql("SELECT encode(csv, 'hex') FROM report_outbox WHERE published_at IS NULL AND file_id = '" + file_id + "';")
            assert bytes.fromhex(stored) == payload, "pending task changed after restart"
        print("OK: report accepted with Kafka offline; report + CSV persisted through PostgreSQL and reports restarts", flush=True)
    finally:
        compose("start", "postgres", *REPORTS, *BROKERS, "nginx")
    healthy(*BROKERS, *REPORTS)
    for file_id, payload in reports:
        for _ in range(180):
            published = sql("SELECT published_at IS NOT NULL AND csv IS NULL FROM report_outbox WHERE file_id = '" + file_id + "';") == "t"
            try:
                csv, _ = request(BASE + "/api/v1/reports/files/" + file_id, token=token)
                if published and csv == payload:
                    break
            except AssertionError:
                pass
            time.sleep(.5)
        else:
            raise AssertionError("pending CSV was not delivered after Kafka recovery")
    file_id, payload = reports[0]
    for _ in range(90):
        if saved_logs(file_id) >= 1:
            break
        time.sleep(.5)
    else:
        raise AssertionError("initial file delivery log missing")
    previous = saved_logs(file_id)
    sql("UPDATE report_outbox SET published_at = NULL, csv = decode('" + payload.hex() + "', 'hex'), available_at = now() WHERE file_id = '" + file_id + "';")
    for _ in range(120):
        if saved_logs(file_id) > previous:
            break
        time.sleep(.5)
    else:
        raise AssertionError("redelivery was not consumed by file service")
    csv, _ = request(BASE + "/api/v1/reports/files/" + file_id, token=token)
    assert csv == payload, "same file ID was not safe to overwrite with identical CSV"
    baseline = Path("/tmp/distributed-cluster-baseline.json")
    if baseline.exists():
        old = json.loads(baseline.read_text())
        csv, _ = request(BASE + "/api/v1/reports/files/" + old["file_id"], token=token)
        assert hashlib.sha256(csv).hexdigest() == old["sha256"], "old CSV changed"
    print("OK: tasks delivered after Kafka recovery; duplicate delivery keeps the same file/content; old CSV preserved", flush=True)


if __name__ == "__main__":
    main()
