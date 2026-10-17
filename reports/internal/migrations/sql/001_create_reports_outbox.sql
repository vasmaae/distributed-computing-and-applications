CREATE TABLE reports (
    file_id uuid PRIMARY KEY,
    total integer NOT NULL CHECK (total >= 0),
    is_fired integer NOT NULL CHECK (is_fired BETWEEN 0 AND total),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE report_outbox (
    file_id uuid PRIMARY KEY REFERENCES reports(file_id),
    csv bytea,
    trace_headers jsonb NOT NULL DEFAULT '{}',
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    last_error text,
    CHECK (
        (published_at IS NULL AND csv IS NOT NULL AND octet_length(csv) BETWEEN 1 AND 8388608)
        OR (published_at IS NOT NULL AND csv IS NULL)
    )
);

CREATE INDEX report_outbox_pending ON report_outbox (available_at, file_id)
WHERE published_at IS NULL;

---- create above / drop below ----

DROP TABLE report_outbox;
DROP TABLE reports;
