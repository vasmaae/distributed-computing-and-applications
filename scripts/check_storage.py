"""Check physical S3 replicas, data preservation and a volume-server outage."""

from collections import defaultdict
import hashlib
import json
from pathlib import Path
import time

from check_instances import compose
from smoke import BASE, read_env, request


NODES = {"seaweedfs:9340", "seaweedfs-volume-2:8080", "seaweedfs-volume-3:8080"}


def storage_json(url):
    return json.loads(compose("exec", "-T", "seaweedfs", "wget", "-qO-", "--header=Accept: application/json", url))


def nodes():
    topology = storage_json("http://localhost:9333/dir/status")["Topology"]
    return {node["Url"] for dc in topology["DataCenters"] for rack in dc["Racks"] for node in rack["DataNodes"]}


def replicas():
    copies = defaultdict(set)
    for node in NODES:
        for volume in storage_json("http://" + node + "/status")["Volumes"] or []:
            assert volume["ReplicaPlacement"] == {"node": 1}, "volume replication must be 001"
            copies[volume["Id"]].add(node)
    assert copies and all(len(locations) == 2 for locations in copies.values()), "each volume needs two copies on different servers"
    return copies


def chunks(file_id):
    bucket = read_env("storage/.env")["S3_BUCKET"]
    metadata = storage_json("http://localhost:8888/buckets/" + bucket + "/" + file_id + "?metadata=true")
    assert metadata["chunks"], "CSV must be stored on volume servers"
    return {int(chunk["file_id"].split(",", 1)[0]) for chunk in metadata["chunks"]}


def main():
    assert nodes() == NODES, "three registered volume servers required"
    replicas()
    credentials = read_env("employees/.env")
    body, _ = request(BASE + "/api/v1/auth/login", data={"username": credentials["AUTH_USERNAME"], "password": credentials["AUTH_PASSWORD"]})
    token = json.loads(body)["access_token"]

    def download(file_id):
        return request(BASE + "/api/v1/reports/files/" + file_id, token=token)[0]

    def report():
        body, _ = request(BASE + "/api/v1/reports/employees-report", token=token)
        file_id = json.loads(body)["file_id"]
        for _ in range(90):
            try:
                return file_id, download(file_id)
            except AssertionError:
                time.sleep(.5)
        raise AssertionError("report was not stored in replicated S3")

    file_id, csv = report()
    copies = replicas()
    locations = set.union(*(copies[volume] for volume in chunks(file_id)))
    server = next(node.split(":")[0] for node in sorted(locations) if node.startswith("seaweedfs-volume-"))
    baseline = Path("/tmp/distributed-cluster-baseline.json")
    if baseline.exists():
        old = json.loads(baseline.read_text())
        assert hashlib.sha256(download(old["file_id"])).hexdigest() == old["sha256"], "old S3 object changed"
        assert all(len(copies[volume]) == 2 for volume in chunks(old["file_id"])), "old object was not replicated"
    try:
        compose("stop", server)
        for _ in range(40):
            if server + ":8080" not in nodes():
                break
            time.sleep(.5)
        else:
            raise AssertionError("stopped volume server remains registered")
        assert download(file_id) == csv, "existing CSV cannot be read from its other replica"
        report()
        print("OK: 3 volume servers, 2 physical copies per volume; old CSV preserved; read/write works with " + server + " stopped", flush=True)
    finally:
        compose("start", server)
    for _ in range(40):
        if nodes() == NODES:
            break
        time.sleep(.5)
    else:
        raise AssertionError("volume server did not re-register")
    replicas()
    print("OK: restarted volume server registered again; every volume has two copies", flush=True)


if __name__ == "__main__":
    main()
