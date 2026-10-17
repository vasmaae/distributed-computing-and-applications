"""Check replicated topic/history and report delivery with one Kafka node stopped."""

import hashlib
import json
from pathlib import Path
import re
import subprocess
import time

from check_instances import compose
from smoke import BASE, read_env, request


BOOTSTRAP = "kafka-1:9092,kafka-2:9092,kafka-3:9092"


def kafka(command, *args):
    return compose("exec", "-T", "kafka-2", "/opt/kafka/bin/" + command + ".sh", "--bootstrap-server", BOOTSTRAP, *args)


def partitions(topic):
    description = kafka("kafka-topics", "--describe", "--topic", topic)
    return [dict(re.findall(r"(Leader|Replicas|Isr):\s*([\d,-]+)", line)) for line in description.splitlines() if "Partition:" in line]


def main():
    status = kafka("kafka-metadata-quorum", "describe", "--status")
    voters = json.loads(next(line.split(":", 1)[1] for line in status.splitlines() if line.startswith("CurrentVoters:")))
    assert {v["id"] for v in voters} == {1, 2, 3}, "three voting controllers required"
    assert all(v["endpoints"] == [f"CONTROLLER://kafka-{v['id']}:9093"] for v in voters), "controller addresses must survive container replacement"
    for topic in ("reports.files", "__consumer_offsets"):
        entries = partitions(topic)
        assert entries and all(set(e["Replicas"].split(",")) == {"1", "2", "3"} and set(e["Isr"].split(",")) == {"1", "2", "3"} for e in entries), topic
    assert len(partitions("reports.files")) >= 3
    config = kafka("kafka-configs", "--describe", "--entity-type", "topics", "--entity-name", "reports.files")
    assert "min.insync.replicas=2" in config
    credentials = read_env("employees/.env")
    body, _ = request(BASE + "/api/v1/auth/login", data={"username": credentials["AUTH_USERNAME"], "password": credentials["AUTH_PASSWORD"]})
    token = json.loads(body)["access_token"]
    baseline = Path("/tmp/distributed-cluster-baseline.json")
    if baseline.exists():
        old = json.loads(baseline.read_text())
        csv, _ = request(BASE + "/api/v1/reports/files/" + old["file_id"], token=token)
        assert hashlib.sha256(csv).hexdigest() == old["sha256"], "old CSV changed"
        consumed = subprocess.run(["docker", "compose", "exec", "-T", "kafka-2", "/opt/kafka/bin/kafka-console-consumer.sh", "--bootstrap-server", BOOTSTRAP, "--topic", "reports.files", "--from-beginning", "--timeout-ms", "5000", "--property", "print.key=true", "--property", "print.value=false"], capture_output=True, text=True)
        assert old["file_id"] in consumed.stdout, "existing Kafka messages lost"
    try:
        compose("stop", "kafka-1")
        for _ in range(30):
            try:
                entries = partitions("reports.files")
                if entries and all(e["Leader"] in ("2", "3") and {"2", "3"} <= set(e["Isr"].split(",")) for e in entries):
                    break
            except subprocess.CalledProcessError:
                pass
            time.sleep(1)
        else:
            raise AssertionError("partition leadership did not recover")
        body, _ = request(BASE + "/api/v1/reports/employees-report", token=token)
        report = json.loads(body)
        for _ in range(60):
            try:
                request(BASE + "/api/v1/reports/files/" + report["file_id"], token=token)
                break
            except AssertionError:
                time.sleep(.5)
        else:
            raise AssertionError("report was not stored with one broker offline")
        print("OK: 3-node quorum, RF=3, min ISR=2; existing history retained; reports delivered with kafka-1 stopped", flush=True)
    finally:
        compose("start", "kafka-1")
    for _ in range(60):
        if all(set(e["Isr"].split(",")) == {"1", "2", "3"} for e in partitions("reports.files")):
            break
        time.sleep(1)
    else:
        raise AssertionError("restarted broker did not rejoin ISR")
    print("OK: restarted broker rejoined ISR", flush=True)


if __name__ == "__main__":
    main()
