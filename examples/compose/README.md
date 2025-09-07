سه‌نودی با Docker Compose

پیش‌نیاز: Docker و docker-compose

دستورها:

1) ساخت و اجرا:

```
docker compose -f examples/compose/docker-compose.yml up --build
```

2) بررسی وضعیت و متریک‌ها:

```
curl http://localhost:17946/status
curl http://localhost:17946/metrics
# نمونه متریک‌های Replication (در صورت فعال بودن مسیر Publish/Subscribe):
# go_cluster_repl_published_total{topic="demo.topic"}
# go_cluster_repl_broadcast_total{topic="demo.topic"}
# go_cluster_repl_acks_total{topic="demo.topic"}
# go_cluster_repl_seq{topic="demo.topic"}
# go_cluster_repl_ack_seq{topic="demo.topic"}
# go_cluster_repl_lag{topic="demo.topic"}
# go_cluster_repl_subs
```

3) توقف:

```
docker compose -f examples/compose/docker-compose.yml down -v
```
