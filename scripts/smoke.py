"""Check the running Compose stack through its gateway and compare the stored CSV."""

import csv
import io
import json
import subprocess
import urllib.request


def main():
    url = "http://localhost:8080/employees/api/v1/reports/employees-report"
    with urllib.request.urlopen(url, timeout=20) as response:
        report = json.load(response)
    assert report["total"] == len(report["employees"]), report
    assert report["is_fired"] == sum(e["is_fired"] for e in report["employees"]), report
    stored = subprocess.check_output([
        "docker", "compose", "exec", "-T", "files-service", "wget", "-qO-",
        "http://localhost:8080/api/v1/files/" + report["file_id"],
    ], timeout=20).decode()
    rows = list(csv.DictReader(io.StringIO(stored)))
    assert len(rows) == report["total"], rows
    assert {row["id"] for row in rows} == {e["id"] for e in report["employees"]}
    print("OK: gateway → employees → reports → files → S3; CSV matches report")


if __name__ == "__main__":
    main()
