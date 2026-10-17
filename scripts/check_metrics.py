"""Check infrastructure dashboard queries and the metrics of a Kafka-node outage."""

import json
import time
import urllib.parse

from check_instances import compose
from smoke import BASE, read_env, request


def query(expression, *, grafana_auth=None):
    url = "http://localhost:9090/api/v1/query?"
    if grafana_auth:
        url = "http://localhost:3000/api/datasources/proxy/uid/prometheus/api/v1/query?"
    body, _ = request(url + urllib.parse.urlencode({"query": expression}), basic_auth=grafana_auth)
    result = json.loads(body)
    assert result["status"] == "success", expression
    return result["data"]["result"]


def main():
    settings = read_env("employees/.env")
    body, _ = request(BASE + "/api/v1/auth/login", data={"username": settings["AUTH_USERNAME"], "password": settings["AUTH_PASSWORD"]})
    token = json.loads(body)["access_token"]
    body, _ = request(BASE + "/api/v1/reports/employees-report", token=token)
    file_id = json.loads(body)["file_id"]
    for _ in range(60):
        try:
            request(BASE + "/api/v1/reports/files/" + file_id, token=token)
            break
        except AssertionError:
            time.sleep(.5)
    else:
        raise AssertionError("CSV was not delivered")
    for _ in range(40):
        targets = query('up{job=~"kafka|seaweedfs|seaweedfs-volume"}')
        if len(targets) == 4 and all(t["value"][1] == "1" for t in targets):
            break
        time.sleep(.5)
    else:
        raise AssertionError("Kafka exporter and three storage nodes must be scraped")
    assert query('kafka_brokers{job="kafka"}')[0]["value"][1] == "3"
    assert len(query('kafka_topic_partition_in_sync_replica{topic="reports.files"}')) == 3
    assert query('SeaweedFS_s3_bucket_object_count{bucket="files"}'), "S3 bucket metrics missing"
    grafana = read_env("monitoring/.env")
    credentials = (grafana["GF_SECURITY_ADMIN_USER"], grafana["GF_SECURITY_ADMIN_PASSWORD"])
    for uid, count in (("kafka", 6), ("s3", 8)):
        body, _ = request("http://localhost:3000/api/dashboards/uid/" + uid, basic_auth=credentials)
        panels = json.loads(body)["dashboard"]["panels"]
        assert len(panels) == count
        for panel in panels:
            for target in panel["targets"]:
                expression = target["expr"].replace("$__rate_interval", "1m")
                for _ in range(40):
                    if query(expression, grafana_auth=credentials):
                        break
                    time.sleep(.5)
                else:
                    raise AssertionError("dashboard query has no data: " + panel["title"])
        print("OK: Grafana " + uid + " dashboard; every panel queries real Prometheus data", flush=True)
    try:
        compose("stop", "kafka-1")
        for _ in range(60):
            isr = query('kafka_topic_partition_in_sync_replica{topic="reports.files"}')
            under = query('sum(kafka_topic_partition_under_replicated_partition{topic="reports.files"})')
            if len(isr) == 3 and all(t["value"][1] == "2" for t in isr) and under and under[0]["value"][1] == "3":
                break
            time.sleep(1)
        else:
            raise AssertionError("dashboard metrics did not detect broker outage")
        print("OK: broker outage is visible as ISR=2 and 3 under-replicated report partitions", flush=True)
    finally:
        compose("start", "kafka-1")
    for _ in range(60):
        isr = query('kafka_topic_partition_in_sync_replica{topic="reports.files"}')
        if len(isr) == 3 and all(t["value"][1] == "3" for t in isr):
            break
        time.sleep(1)
    else:
        raise AssertionError("recovered broker is not visible in metrics")
    print("OK: metrics show all replicas synchronized after recovery", flush=True)


if __name__ == "__main__":
    main()
