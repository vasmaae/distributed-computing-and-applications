"""Check sorting through the gateway without changing employee data."""

from datetime import datetime
import json
import uuid

from smoke import BASE, read_env, request


def main():
    settings = read_env("employees/.env")
    options = {"sort_by": "id", "order": "asc", "threads": 5}
    url = BASE + "/api/v1/employees/sorted"
    request(url, data=options, status=401)
    body, _ = request(BASE + "/api/v1/auth/login", data={
        "username": settings["AUTH_USERNAME"], "password": settings["AUTH_PASSWORD"],
    })
    token = json.loads(body)["access_token"]
    body, _ = request(BASE + "/api/v1/employees", token=token)
    original = json.loads(body)
    assert isinstance(original, list), "ordinary GET must still return an array"
    expected = {employee["id"]: employee for employee in original}
    keys = {
        "id": lambda employee: uuid.UUID(employee["id"]).bytes,
        "birth_date": lambda employee: datetime.fromisoformat(employee["birth_date"].replace("Z", "+00:00")),
        "fio": lambda employee: " ".join(employee["fio"].lower().split()).replace("ё", "е"),
    }
    for field, key in keys.items():
        for order in ("asc", "desc"):
            timings = []
            for threads in (1, 2, 3, 5, 8):
                body, _ = request(url, token=token, data={
                    "sort_by": field, "order": order, "threads": threads,
                })
                result = json.loads(body)
                employees = result["employees"]
                assert isinstance(employees, list)
                assert len(employees) == len(original)
                assert {employee["id"]: employee for employee in employees} == expected
                values = [key(employee) for employee in employees]
                assert values == sorted(values, reverse=order == "desc"), (field, order, threads)
                elapsed = result["sort_duration_ms"]
                assert isinstance(elapsed, (int, float)) and elapsed >= 0
                timings.append(f"{threads}: {elapsed:.6f}")
            print(f"OK: {field} {order}, {len(original)} employees; threads: ms = " + ", ".join(timings))

    for invalid in (
        {}, {**options, "sort_by": "wrong"}, {**options, "order": "up"},
        {**options, "threads": 0}, {**options, "threads": -1},
        {**options, "threads": 257}, {**options, "threads": "5"},
        {**options, "extra": True},
    ):
        request(url, token=token, data=invalid, status=400)
    print("OK: JWT, request validation, unchanged GET response and employee data")


if __name__ == "__main__":
    main()
