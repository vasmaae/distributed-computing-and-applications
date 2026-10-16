"""Check the running Compose stack through its gateway and compare the stored CSV."""

import csv
import io
import json
import urllib.error
import urllib.request
import time


def main():
    url = "http://localhost:8080/employees/api/v1/reports/employees-report"
    with urllib.request.urlopen(url, timeout=20) as response:
        report = json.load(response)
    assert report["total"] == len(report["employees"]), report
    assert report["is_fired"] == sum(e["is_fired"] for e in report["employees"]), report
    for attempt in range(30):
        try:
            with urllib.request.urlopen("http://localhost:8080/employees/api/v1/reports/files/" + report["file_id"], timeout=10) as response:
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
    assert len(rows) == report["total"], rows
    assert {row["id"] for row in rows} == {e["id"] for e in report["employees"]}
    print("OK: report queued and stored; CSV downloaded through gateway via Go future")


if __name__ == "__main__":
    main()
