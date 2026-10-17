#!/usr/bin/env bash
set -euo pipefail
bin=/opt/kafka/bin
bootstrap=kafka-1:9092,kafka-2:9092,kafka-3:9092

for id in 2 3; do
  voters=$("$bin/kafka-metadata-quorum.sh" --bootstrap-server "$bootstrap" describe --status | grep "^CurrentVoters:")
  if ! grep -Eq '"id":[[:space:]]*'"$id"'[,}]' <<< "$voters"; then
    cat > "/tmp/controller-$id.properties" <<PROPERTIES
node.id=$id
process.roles=controller
listeners=CONTROLLER://kafka-$id:9093
controller.listener.names=CONTROLLER
listener.security.protocol.map=CONTROLLER:PLAINTEXT
controller.quorum.bootstrap.servers=kafka-1:9093,kafka-2:9093,kafka-3:9093
metadata.log.dir=/nodes/$id
PROPERTIES
    "$bin/kafka-metadata-quorum.sh" --bootstrap-server "$bootstrap" \
      --command-config "/tmp/controller-$id.properties" add-controller
  fi
done

"$bin/kafka-topics.sh" --bootstrap-server "$bootstrap" --create --if-not-exists \
  --topic "$QUEUE_TOPIC" --partitions "$QUEUE_PARTITIONS" --replication-factor 3 \
  --config max.message.bytes=9437184 --config min.insync.replicas=2
count=$("$bin/kafka-topics.sh" --bootstrap-server "$bootstrap" --describe --topic "$QUEUE_TOPIC" |
  awk '{for(i=1;i<=NF;i++) if($i=="PartitionCount:") print $(i+1)}')
if [ "$count" -lt "$QUEUE_PARTITIONS" ]; then
  "$bin/kafka-topics.sh" --bootstrap-server "$bootstrap" --alter --topic "$QUEUE_TOPIC" --partitions "$QUEUE_PARTITIONS"
fi

# Existing report and offset partitions must gain replicas too; defaults only affect new topics.
printf '{"version":1,"partitions":[' > /tmp/replicas.json
separator=
topics=$("$bin/kafka-topics.sh" --bootstrap-server "$bootstrap" --list)
while IFS= read -r topic; do
  [ -n "$topic" ] || continue
  "$bin/kafka-configs.sh" --bootstrap-server "$bootstrap" --alter --entity-type topics \
    --entity-name "$topic" --add-config min.insync.replicas=2
  partitions=$("$bin/kafka-topics.sh" --bootstrap-server "$bootstrap" --describe --topic "$topic" |
    awk '{for(i=1;i<=NF;i++) if($i=="Partition:") print $(i+1)}')
  for partition in $partitions; do
    case $((partition % 3)) in
      0) replicas=1,2,3 ;;
      1) replicas=2,3,1 ;;
      2) replicas=3,1,2 ;;
    esac
    printf '%s{"topic":"%s","partition":%s,"replicas":[%s]}' "$separator" "$topic" "$partition" "$replicas" >> /tmp/replicas.json
    separator=,
  done
done <<< "$topics"
printf ']}' >> /tmp/replicas.json
"$bin/kafka-reassign-partitions.sh" --bootstrap-server "$bootstrap" --execute --reassignment-json-file /tmp/replicas.json
for attempt in $(seq 1 60); do
  status=$("$bin/kafka-reassign-partitions.sh" --bootstrap-server "$bootstrap" --verify --reassignment-json-file /tmp/replicas.json)
  if ! grep -q 'in progress' <<< "$status"; then
    printf '%s\n' "$status"
    "$bin/kafka-leader-election.sh" --bootstrap-server "$bootstrap" --election-type PREFERRED --all-topic-partitions
    exit 0
  fi
  sleep 2
done
printf '%s\n' 'Kafka replica assignment did not finish' >&2
exit 1
