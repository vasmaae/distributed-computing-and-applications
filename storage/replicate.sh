#!/bin/sh
set -eu

# Changing defaults alone leaves existing volumes without a second copy.
{
  printf '%s\n' 'lock' \
    'fs.configure -locationPrefix=/buckets/ -replication=001 -volumeGrowthCount=6 -apply' \
    'volume.configure.replication -replication=001'
  sleep 5 # Allow changed placement to reach the master's topology via heartbeats.
  printf '%s\n' 'volume.fix.replication -apply -doDelete=false'
  sleep 5
  printf '%s\n' 'volume.balance -apply' 'unlock'
} | weed shell -master=seaweedfs:9333 -filer=seaweedfs:8888

topology=$(printf 'volume.list\n' | weed shell -master=seaweedfs:9333)
printf '%s\n' "$topology"
printf '%s\n' "$topology" | awk '
  /volume Id:/ {
    id=$0; sub(/^.*volume Id:/, "", id); sub(/,.*/, "", id)
    copies[id]++
    if ($0 !~ /ReplicaPlacement:001,/) bad=1
  }
  END {
    for (id in copies) if (copies[id] != 2) bad=1
    exit bad
  }
'
