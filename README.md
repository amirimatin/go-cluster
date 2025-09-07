# go-cluster

نسخه‌ی اولیه از پکیج خوشه‌بندی (membership + leader election + transport) بر اساس سند طراحی `go-cluster.md`.

- برنامه‌ی توسعه: `DEVELOPMENT_PLAN.md`
- باینری نمونه: `cmd/clusterctl` (CLI نازک و مصرف‌کننده‌ی بسته)
- مثال‌ها: `examples/`
 - آموزش کامل ایجاد کلاستر ۳ نودی در میکروسرویس: `docs/tutorials/ThreeNodeMicroservice.md`
 - راهنمای مانیتورینگ/PromQL/داشبورد: `docs/monitoring/Monitoring.md`

## راه‌اندازی سریع (Quick Start)

- پیش‌نیاز: Go 1.22+، پورت‌های خالی لوکال برای `raft`, `membership`, `mgmt`.
- ساخت و تست:
  - `make tidy && make build`
  - `make test` و در صورت نیاز `make test-integration`
- اجرای 3 نود لوکال:
  - ترمینال 1 (لیدر):
    - `go run ./cmd/clusterctl run --id n1 --raft-addr :9521 --mem-bind :7946 --mgmt-addr :17946 --bootstrap`
  - ترمینال 2:
    - `go run ./cmd/clusterctl run --id n2 --raft-addr :9522 --mem-bind :8946 --mgmt-addr :18946 --join 127.0.0.1:7946`
  - ترمینال 3:
    - `go run ./cmd/clusterctl run --id n3 --raft-addr :9523 --mem-bind :9946 --mgmt-addr :19946 --join 127.0.0.1:7946`
  - افزودن به RAFT از طریق مدیریت (روی لیدر):
    - `go run ./cmd/clusterctl join --id n2 --raft-addr 127.0.0.1:9522 --addr 127.0.0.1:17946`
    - `go run ./cmd/clusterctl join --id n3 --raft-addr 127.0.0.1:9523 --addr 127.0.0.1:17946`
  - بررسی وضعیت و متریک‌ها:
    - `curl http://127.0.0.1:17946/status`
    - `curl http://127.0.0.1:17946/metrics`


## ساخت و اجرا

```
make tidy
make build
make test            # تست‌های واحد
make test-integration # تست‌های ادغامی (نیازمند پورت‌های خالی لوپ‌بک)
```

برای اجرای نمونه‌ی hello:
```
go run ./examples/hello
```

برای اجرای اسکلت CLI (زیرفرمان run):
```
# اجرای یک نود با شناسه و آدرس‌ها (توجه: پورت مدیریت را جدا از پورت عضویت قرار دهید)
go run ./cmd/clusterctl run \
  --id node1 \
  --raft-addr :9520 \
  --mem-bind :7946 \
  --mgmt-addr :17946 \
  --join ""
```

راه‌اندازی کلاستر سه‌نودی (Dev):
```
# ترمینال 1: نود لیدر با Bootstrap
go run ./cmd/clusterctl run --id n1 --raft-addr :9521 --mem-bind :7946 --mgmt-addr :17946 --bootstrap

# ترمینال 2: نود دوم، به نود اول (seed) معرفی می‌شود
go run ./cmd/clusterctl run --id n2 --raft-addr :9522 --mem-bind :8946 --mgmt-addr :18946 --join 127.0.0.1:7946

# ترمینال 3: نود سوم
go run ./cmd/clusterctl run --id n3 --raft-addr :9523 --mem-bind :9946 --mgmt-addr :19946 --join 127.0.0.1:7946

# بررسی وضعیت از هر نود
go run ./cmd/clusterctl status --addr 127.0.0.1:17946

# مدیریت از راه دور (join/leave) با خروجی JSON
```
# Join: درخواست افزودن نود به کلاستر از طریق نود لیدر/عضو
go run ./cmd/clusterctl join \
  --id n2 --raft-addr 10.0.0.22:9522 \
  --addr 127.0.0.1:17946 --mgmt-proto http

# Leave: درخواست حذف یک نود (فقط توسط لیدر پذیرفته می‌شود)
go run ./cmd/clusterctl leave \
  --id n2 \
  --addr 127.0.0.1:17946 --mgmt-proto http
```

متریک‌ها (Prometheus)
```
# endpoint متریک‌ها روی هر نود (روی پورت مدیریت):
curl http://127.0.0.1:17946/metrics

# نمونه متریک‌ها:
# go_cluster_members_total, go_cluster_leader_changes_total, go_cluster_join_requests_total{result="accepted"}
```

گرفتن وضعیت کلاستر به‌صورت JSON از طریق زیرفرمان `status`:
```
# آدرس مدیریتی نودی که در حال اجراست را بدهید (پیش‌فرض: 127.0.0.1:17946)
go run ./cmd/clusterctl status --addr 127.0.0.1:17946
```

نکته: آدرس مدیریت هر نود در متادیتای عضویت حمل می‌شود و نودهای غیرلیدر برای برگرداندن وضعیت نهایی، درخواست را به لیدر پروکسی می‌کنند (LeaderAddr در خروجی). حتماً پورت مدیریت (`--mgmt-addr`) را جدا از پورت عضویت تنظیم کنید.

استفاده از gRPC برای مدیریت داخلی (اختیاری):
```
# اجرای نودها با gRPC به‌جای HTTP برای مدیریت داخلی
go run ./cmd/clusterctl run --id n1 --raft-addr :9521 --mem-bind :7946 --mgmt-addr :17946 --mgmt-proto grpc --bootstrap

# توجه: endpoint های HTTP مانند /status و /metrics مربوط به HTTP/JSON هستند؛
# برای gRPC، `status` داخلی از طریق proxy انجام می‌شود. برای مشاهده HTTP/metrics، از حالت HTTP استفاده کنید.
```

ابزار موقت برای تست membership (دو ترمینال اجرا کنید و به هم join دهید):
```
go run ./cmd/memdemo -id n1 -bind :7946
go run ./cmd/memdemo -id n2 -bind :8946 -join 127.0.0.1:7946
```

نکته: ترنسپورت مدیریتی HTTP/JSON و gRPC در دسترس است؛ mTLS برای مدیریت از طریق فلگ‌های `--tls-*` قابل‌فعال‌سازی است. برای پایداری داده‌ها، اگر `-data` تنظیم شود، Raft از boltDB بر روی دیسک استفاده می‌کند.

## استفاده به‌عنوان کتابخانه (Reusable Entry Point)

برای استفاده در میکروسرویس‌ها، از پکیج `pkg/bootstrap` استفاده کنید. این پکیج همه‌ی مؤلفه‌ها را بر اساس یک `Config` مونتاژ می‌کند و یک `*cluster.Cluster` آماده‌ی اجرا در اختیار شما می‌گذارد.

نمونه ادغام ساده در سرویس شما:

```
package main

import (
  "context"
  "log"
  "os/signal"
  "syscall"

  "github.com/amirimatin/go-cluster/pkg/bootstrap"
)

func main() {
  ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
  defer cancel()

  // اختیاری: فعال‌سازی Tracing (خروجی به stdout برای توسعه)
  // shutdown, _ := tracing.Setup(true)
  // defer shutdown(context.Background())

  cl, err := bootstrap.Run(ctx, bootstrap.Config{
    NodeID:   "svc-1",
    RaftAddr: ":9521",
    MemBind:  ":7946",
    MgmtAddr: ":17946", // پورت مدیریت جدا از پورت عضویت
    DiscoveryKind: "static",
    SeedsCSV:      "127.0.0.1:7946",
    DataDir:  "/var/lib/myservice/raft",
    Bootstrap: false,
  })
  if err != nil { log.Fatalf("cluster bootstrap: %v", err) }
  defer cl.Close()

  // ... سرویس خود را اجرا کنید ...
  <-ctx.Done()
}
```

## SDK برنامه (در دست توسعه)

- Event Bus: دریافت رویدادهای `LeaderChanged`, `MemberJoin/Leave/Failed` به‌صورت کانال:
  - `evs := cl.Subscribe(ctx); go func(){ for e := range evs { /* handle */ } }()`
- App Handlers (SPI): پیاده‌سازی `HandleWrite/HandleRead/HandleSync` و تزریق به `bootstrap.Config`.
- AppWrite (مرحله اول): فراخوانی `cl.AppWrite(ctx, op, data)`؛ اگر نود لیدر نباشد، درخواست به لیدر فوروارد می‌شود و فقط در لیدر اجرا می‌شود.
- Replication Channel (استریم gRPC): انتشار پیام‌های `topic+blob` از لیدر به نودها با استریم پایدار (keepalive + اتصال reuse).
  - `Publish(topic, data)` در لیدر → استریم به مشترکین.
  - پشتیبانی `Seq/Ack`: هر پیام دارای Sequence است؛ کلاینت‌ها Ack می‌فرستند. متریک‌ها: `go_cluster_repl_{published_total,broadcast_total,acks_total}`, `go_cluster_repl_seq`, `go_cluster_repl_ack_seq`, `go_cluster_repl_lag`, `go_cluster_repl_subs`.

سناریو نمونه (به‌زودی):
```
type MyHandlers struct{ /* ... */ }
// func (MyHandlers) HandleWrite(ctx context.Context, op string, req []byte) ([]byte, error) { /* ... */ }
// func (MyHandlers) HandleRead(ctx context.Context, op string, req []byte) ([]byte, error)  { /* ... */ }
// func (MyHandlers) HandleSync(ctx context.Context, topic string, data []byte) error        { /* ... */ }
```

نکته: CLI صرفاً برای توسعه/تست است و منطق کلاستر در بسته‌ها قرار دارد (اصل وظیفه واحد).

## الحاق CLI در میکروسرویس‌ها (Embeddable CLI)

اگر سرویس شما از Cobra استفاده می‌کند، می‌توانید زیرفرمان‌های کلاستر را به‌سادگی به ریشهٔ CLI خود اضافه کنید. این کار به شما اجازه می‌دهد بدون باینری جداگانه، دستورات `run/status/join/leave` را در سرویس خود داشته باشید و از طریق فلگ‌ها با پکیج تعامل کانفیگی داشته باشید.

گزینه ۱: افزودن یک فرمان والد `cluster` با زیرفرمان‌ها:

```
package main

import (
  "log"
  "github.com/spf13/cobra"
  clustercli "github.com/amirimatin/go-cluster/pkg/cli"
)

func main() {
  root := &cobra.Command{Use: "myservice"}
  // اضافه کردن فرمان "cluster" با زیرفرمان‌های run/status/join/leave
  root.AddCommand(clustercli.NewClusterCommand())
  if err := root.Execute(); err != nil { log.Fatal(err) }
}
```

گزینه ۲: افزودن مستقیم زیرفرمان‌ها به ریشهٔ سرویس:

```
root := &cobra.Command{Use: "myservice"}
clustercli.AddAll(root) // دستورات run/status/join/leave مستقیماً به ریشه افزوده می‌شوند
```

پیکربندی: تمام فلگ‌های مدیریت (مانند `--id`, `--raft-addr`, `--mem-bind`, `--mgmt-addr`, `--discovery`, `--tls-*`) همانند باینری `clusterctl` در دسترس خواهند بود و می‌توانید با defaults پروژه خود (از طریق Cobra یا لایهٔ config خود) هماهنگ‌شان کنید. برای اجرای درون‌فرایندی بدون CLI نیز از `bootstrap.Config` استفاده کنید.

## Docker Compose (سه‌نودی)

برای اجرای نمونه‌ی سه‌نودی با Docker:

```
docker compose -f examples/compose/docker-compose.yml up --build

# وضعیت و متریک‌ها
curl http://localhost:17946/status
curl http://localhost:17946/metrics

# توقف و پاکسازی
docker compose -f examples/compose/docker-compose.yml down -v
```

## Discovery پیشرفته

نمونه‌های استفاده از DNS و فایل/ENV برای کشف seed ها:

```
# استفاده از DNS SRV/A برای کشف seed ها
go run ./cmd/clusterctl run --id n1 --raft-addr :9521 --mem-bind :7946 --mgmt-addr :17946 \
  --discovery dns --dns-names _cluster._tcp.example.com --dns-port 7946 --disc-refresh 5s --bootstrap

# استفاده از فایل/ENV برای seed ها
echo "127.0.0.1:7946,127.0.0.1:8946" > /tmp/seeds.txt
go run ./cmd/clusterctl run --id n2 --raft-addr :9522 --mem-bind :8946 --mgmt-addr :18946 \
  --discovery file --file-path /tmp/seeds.txt --disc-refresh 5s

# یا با ENV (اولویت بالاتر از فایل):
export CLUSTER_SEEDS=127.0.0.1:7946,127.0.0.1:8946
go run ./cmd/clusterctl run --id n3 --raft-addr :9523 --mem-bind :9946 --mgmt-addr :19946 \
  --discovery file --file-path /tmp/any.txt --file-env CLUSTER_SEEDS
```

## TLS/mTLS (مدیریت)

برای فعال‌سازی TLS یا mTLS در ترنسپورت مدیریتی (HTTP/JSON یا gRPC):

```
# اجرای نود با TLS سمت سرور (گواهی سرور)
go run ./cmd/clusterctl run --id n1 \
  --raft-addr :9521 --mem-bind :7946 --mgmt-addr :17946 \
  --tls-enable --tls-cert server.crt --tls-key server.key --tls-ca ca.crt --bootstrap

# فراخوانی join با mTLS از کلاینت
go run ./cmd/clusterctl join --id n2 --raft-addr 127.0.0.1:9522 \
  --addr 127.0.0.1:17946 --tls-enable --tls-ca ca.crt --tls-cert client.crt --tls-key client.key

# توجه: در حالت HTTP، scheme به‌صورت خودکار به https تغییر می‌کند وقتی TLS فعال باشد.
```

در حالت توسعه می‌توانید با `--tls-skip-verify` اعتبارسنجی گواهی سرور را غیرفعال کنید (توصیه نمی‌شود در تولید).

## انتشار (Release)

- نسخه‌گذاری: از SemVer پیروی کنید (مثلاً `v0.1.0`).
- فرآیند:
  - تغییرات را در `CHANGELOG.md` ثبت کنید.
  - تگ بسازید: `git tag -a v0.1.0 -m "v0.1.0" && git push origin v0.1.0`
  - GitHub Actions (`.github/workflows/release.yml`) به‌صورت خودکار باینری‌های `clusterctl` را برای پلتفرم‌های رایج می‌سازد و به Release پیوست می‌کند.
## قابلیت اطمینان Replication (فاز 14)

- at-least-once: پیام‌ها دارای `seq` هستند و کلاینت‌ها Ack ارسال می‌کنند؛ لیدر بافر in-memory نگه‌داری می‌کند و پیام‌های Unacked را بازارسال می‌کند.
- backpressure/window: اگر `seq - minAck > window` (پیش‌فرض 1024)، `Publish` تا رسیدن Ack مسدود/کند می‌شود.
- اتصال‌ها: استریم gRPC پایدار با keepalive + ConnManager برای reuse اتصالات.

پیکربندی (bootstrap.Config):
- `ReplWindow` (int): حداکثر پنجره Unacked قبل از backpressure. پیش‌فرض: 1024
- `ReplRetry` (duration): بازه بازارسال پیام‌های Unacked در لیدر. پیش‌فرض: 1.5s
- `ReplBufferDir` (string): مسیر ذخیره‌سازی دیسکی برای بافر پیام‌های Unacked (برای بازیابی پس از ری‌استارت).

پیکربندی (اختیاری از طریق bootstrap.Config):
- `ReplWindow` (int): حداکثر پنجره Unacked قبل از backpressure. پیش‌فرض: 1024
- `ReplRetry` (duration): بازه بازارسال پیام‌های Unacked در لیدر. پیش‌فرض: 1.5s

## رصد Replication با Prometheus

- متریک‌های کلیدی:
  - `go_cluster_repl_published_total{topic}`: تعداد publish های لیدر
  - `go_cluster_repl_broadcast_total{topic}`: تعداد پیام‌های ارسال‌شده روی استریم‌ها
  - `go_cluster_repl_acks_total{topic}`: تعداد Ack ها
  - `go_cluster_repl_seq{topic}`, `go_cluster_repl_ack_seq{topic}`, `go_cluster_repl_lag{topic}`: وضعیت Seq/Ack و Lag
  - `go_cluster_repl_subs`: تعداد مشترکین فعال
  - per-node: `go_cluster_repl_acks_per_node_total{topic,node}`, `go_cluster_repl_ack_seq_per_node{topic,node}`, `go_cluster_repl_lag_per_node{topic,node}`

- نمونه PromQL:
  - نرخ publish در 5 دقیقه اخیر: `rate(go_cluster_repl_published_total[5m])`
  - lag فعلی به ازای topic: `go_cluster_repl_lag`
- نرخ Ack به ازای topic: `rate(go_cluster_repl_acks_total[5m])`

بیشتر بخوانید: `docs/monitoring/Monitoring.md` (نمونه پرس‌وجوها، هشدارها، و داشبورد Grafana در `docs/monitoring/grafana-replication.json`).
