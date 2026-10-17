#!/usr/bin/env bash
set -euo pipefail

# The 4.1 Docker wrapper only formats static quorums; dynamic membership needs native formatting.
config=/tmp/server.properties
env | LC_ALL=C sort | awk -F= '
  /^KAFKA_/ && $1 !~ /^KAFKA_(HEAP_OPTS|JVM_PERFORMANCE_OPTS|OPTS|LOG4J_OPTS)$/ {
    key = substr($1, 7)
    gsub(/_/, ".", key)
    print tolower(key) "=" substr($0, index($0, "=") + 1)
  }
' > "$config"

if [ ! -s "$KAFKA_LOG_DIRS/meta.properties" ]; then
  mode=--no-initial-controllers
  if [ "$KAFKA_NODE_ID" = 1 ]; then mode=--standalone; fi
  /opt/kafka/bin/kafka-storage.sh format --cluster-id "$CLUSTER_ID" --config "$config" "$mode"
fi
exec /opt/kafka/bin/kafka-server-start.sh "$config"
