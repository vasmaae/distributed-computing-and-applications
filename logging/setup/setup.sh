#!/usr/bin/env bash
set -euo pipefail

curl --fail-with-body --silent --show-error --retry 10 --retry-delay 1 --retry-connrefused --max-time 15 \
  -X PUT http://elasticsearch:9200/_index_template/app-logs \
  -H 'Content-Type: application/json' --data-binary @/setup/index-template.json

curl --fail-with-body --silent --show-error --retry 10 --retry-delay 1 --retry-connrefused --max-time 15 \
  -X POST http://kibana:5601/api/data_views/data_view \
  -H 'kbn-xsrf: setup' -H 'Content-Type: application/json' \
  --data-binary @/setup/data-view.json

curl --fail-with-body --silent --show-error --retry 10 --retry-delay 1 --retry-connrefused --max-time 15 \
  -X POST http://kibana:5601/api/data_views/default \
  -H 'kbn-xsrf: setup' -H 'Content-Type: application/json' \
  --data-binary '{"data_view_id":"app-logs","force":true}'
