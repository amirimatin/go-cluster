# Monitoring go-cluster

This guide provides practical Prometheus queries, sample Grafana dashboard, and alerting suggestions for monitoring replication reliability and cluster health.

## Metrics overview

- Cluster health:
  - `go_cluster_members_total`
  - `go_cluster_is_leader`
  - `go_cluster_leader_changes_total`
  - `go_cluster_join_requests_total{result}`
- Replication (topic):
  - `go_cluster_repl_published_total{topic}`
  - `go_cluster_repl_broadcast_total{topic}`
  - `go_cluster_repl_acks_total{topic}`
  - `go_cluster_repl_seq{topic}`
  - `go_cluster_repl_ack_seq{topic}`
  - `go_cluster_repl_lag{topic}`
  - `go_cluster_repl_subs`
- Replication (per-node):
  - `go_cluster_repl_acks_per_node_total{topic,node}`
  - `go_cluster_repl_ack_seq_per_node{topic,node}`
  - `go_cluster_repl_lag_per_node{topic,node}`

## PromQL examples

- Publish rate (5m):
  - `sum by (topic) (rate(go_cluster_repl_published_total[5m]))`
- Broadcast rate (5m):
  - `sum by (topic) (rate(go_cluster_repl_broadcast_total[5m]))`
- Ack rate (5m):
  - `sum by (topic) (rate(go_cluster_repl_acks_total[5m]))`
- Topic lag (seq - minAck):
  - `go_cluster_repl_lag`
- Per-node lag:
  - `go_cluster_repl_lag_per_node`
- Active subscribers:
  - `go_cluster_repl_subs`

## Alerts (suggestions)

- High lag sustained (10m):
```
- alert: ReplicationLagHigh
  expr: go_cluster_repl_lag > 1000
  for: 10m
  labels: {severity: warning}
  annotations:
    summary: "High replication lag"
    description: "Topic {{ $labels.topic }} lag is {{ $value }} (>1000) for 10m"
```

- No subscribers (leader):
```
- alert: ReplicationNoSubscribers
  expr: go_cluster_repl_subs == 0 and on() go_cluster_is_leader == 1
  for: 5m
  labels: {severity: critical}
  annotations:
    summary: "No replication subscribers on leader"
```

## Grafana dashboard

A sample dashboard JSON is available at `docs/monitoring/grafana-replication.json`. Import it into Grafana and select your Prometheus data source.

## Tuning

- `ReplWindow` (default 1024): increase for higher throughput if your consumers keep up; reduce if memory is constrained.
- `ReplRetry` (default 1.5s): reduce for faster catch-up at the cost of extra traffic; increase to reduce pressure.
- `ReplBufferDir`: set to a local fast disk path to persist unacked messages for leader restart recovery.

