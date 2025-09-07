# راهنمای جامع استفاده از go-cluster در یک میکروسرویس و ایجاد کلاستر ۳ نودی

این راهنما به‌صورت گام‌به‌گام نشان می‌دهد چگونه پکیج `go-cluster` را در یک میکروسرویس ادغام کنید و یک کلاستر ۳ نودی با قابلیت‌های کامل (عضویت، لیدرشیپ/RAFT، مدیریت، TLS/mTLS، Event Bus، Replication با Seq/Ack، متریک‌ها و observability) بسازید.

## پیش‌نیازها
- Go 1.22+ (module: `github.com/amirimatin/go-cluster`)
- پورت‌های آزاد روی لوپ‌بک یا شبکه شما:
  - RAFT: به‌ازای هر نود یک پورت (مثلاً 9521/9522/9523)
  - Membership (memberlist): 7946/8946/9946
  - Management (HTTP/gRPC): 17946/18946/19946
- (اختیاری) TLS/mTLS: فایل‌های CA/Server/Client cert در محیط‌های امن
- سیستم‌عامل/فایروال: دسترسی بین نودها روی پورت‌های فوق

## نصب پکیج
```
# در go.mod سرویس شما
require github.com/amirimatin/go-cluster latest

# یا با نسخه مشخص (پس از انتشار ریلیز)
# require github.com/amirimatin/go-cluster v0.1.0
```

## مفاهیم کلیدی
- Membership (memberlist): کشف و رویدادهای Join/Leave/Failed
- Consensus (RAFT): انتخاب لیدر و commit تغییرات عضویت
- Management API: HTTP/JSON یا gRPC برای status/join/leave/metrics/healthz
- Event Bus: رویدادهای LeaderChanged, ElectionStart/End, MemberJoin/Leave/Failed
- App Handlers (SPI): پیاده‌سازی `HandleWrite/HandleRead/HandleSync` توسط سرویس
- Replication (استریم gRPC): `Publish(topic,data)` از لیدر به همه نودها + Seq/Ack و backpressure
- Observability: متریک‌ها (Prometheus) و tracing (OpenTelemetry)

## ادغام در میکروسرویس (نمونه کامل)
فایل `main.go` سرویس شما (نود ۱: لیدر)
```go
package main

import (
  "context"
  "fmt"
  "log"
  "net/http"
  "os/signal"
  "syscall"
  "time"

  "github.com/amirimatin/go-cluster/pkg/bootstrap"
  tracing "github.com/amirimatin/go-cluster/pkg/observability/tracing"
)

// هندلرهای برنامه: سرویس شما payloadها را تفسیر/اجرا می‌کند
type appHandlers struct{}
func (appHandlers) HandleWrite(ctx context.Context, op string, req []byte) ([]byte, error) {
  // فقط روی لیدر اجرا می‌شود (فالوئر فوروارد می‌کند)
  return []byte(fmt.Sprintf("op=%s at=%d payload=%s", op, time.Now().Unix(), string(req))), nil
}
func (appHandlers) HandleRead(ctx context.Context, op string, req []byte) ([]byte, error)  { return []byte("read-ok"), nil }
func (appHandlers) HandleSync(ctx context.Context, topic string, data []byte) error        { log.Printf("SYNC topic=%s data=%s", topic, string(data)); return nil }

func main() {
  // سیگنال‌های خاتمه
  ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
  defer cancel()

  // (اختیاری) Tracing برای توسعه
  shutdown, _ := tracing.Setup(false)
  defer shutdown(context.Background())

  // پیکربندی نود ۱ (لیدر)
  cfg := bootstrap.Config{
    NodeID:        "n1",
    RaftAddr:      "127.0.0.1:9521",
    MemBind:       "127.0.0.1:7946",
    MgmtAddr:      "127.0.0.1:17946",
    MgmtProto:     "http", // یا "grpc"
    DiscoveryKind: "static",
    SeedsCSV:      "",     // لیدر اولیه: خالی
    DataDir:       "/tmp/cluster-n1", // توصیه برای تولید: مسیر دائمی
    Bootstrap:     true,    // نود اول در کلاستر
    // TLS مدیریتی (اختیاری)
    // TLSEnable: true, TLSCA: "ca.crt", TLSCert: "server.crt", TLSKey: "server.key",

    AppHandlers: appHandlers{},
    // تنظیمات Replication (اختیاری)
    ReplWindow:   1024,
    ReplRetry:    1500 * time.Millisecond,
    ReplBufferDir:"/tmp/cluster-n1/replbuf", // اختیاری: بازیابی پس از ری‌استارت

    // Callbackهای انتخاباتی (اختیاری)
    // توجه: نوع LeaderInfo از pkg/consensus اخذ می‌شود
    OnLeaderChange: func(info bootstrap.LeaderInfo) {},
    OnElectionStart: func(){ log.Printf("election started") },
    OnElectionEnd:   func(info bootstrap.LeaderInfo){ log.Printf("election ended leader=%s", info.ID) },
  }

  cl, err := bootstrap.Run(ctx, cfg)
  if err != nil { log.Fatalf("cluster bootstrap: %v", err) }
  defer cl.Close()

  // رویدادها را مشترک شوید (Event Bus)
  evs := cl.Subscribe(ctx)
  go func(){ for e := range evs { log.Printf("EVENT type=%s term=%d", e.Type, e.Term) } }()

  // HTTP ساده سرویس
  http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
  http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request){ s, _ := cl.Status(r.Context()); _, _ = w.Write([]byte(fmt.Sprintf("leader=%s term=%d members=%d\n", s.LeaderID, s.Term, len(s.Members)))) })
  http.HandleFunc("/write", func(w http.ResponseWriter, r *http.Request){ out, err := cl.AppWrite(r.Context(), "demo", []byte("hello")); if err != nil { w.WriteHeader(500); _, _ = w.Write([]byte(err.Error())); return }; _, _ = w.Write(out) })
  http.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request){ if err := cl.Publish(r.Context(), "demo.topic", []byte("sync-data")); err != nil { w.WriteHeader(500); _, _ = w.Write([]byte(err.Error())); return }; _, _ = w.Write([]byte("ok")) })

  log.Println("service-1 listening at :8081 — endpoints: /healthz /status /write /publish")
  _ = http.ListenAndServe(":8081", nil)
}
```

سپس برای نودهای ۲ و ۳، همین برنامه را با پیکربندی مناسب اجرا کنید:
- نود ۲:
  - `NodeID: n2` — `RaftAddr: 127.0.0.1:9522` — `MemBind: 127.0.0.1:8946` — `MgmtAddr: 127.0.0.1:18946`
  - `SeedsCSV: 127.0.0.1:7946` — `Bootstrap: false`
  - مسیر DataDir/BufferDir جداگانه (`/tmp/cluster-n2`, `/tmp/cluster-n2/replbuf`)
- نود ۳: مشابه با پورت‌های `9523/9946/19946` و Seed به `127.0.0.1:7946`

نکته: با `DiscoveryKind: "static"`، فقط گاسیپ/عضویت انجام می‌شود. برای تبدیل نودهای ۲ و ۳ به رأی‌دهنده RAFT باید `join` مدیریتی را روی لیدر صدا بزنید.

## افزودن نودها به RAFT (Join مدیریتی)
پس از بالا آمدن هر سه نود:
```
# افزودن n2 به RAFT از طریق لیدر (نود ۱ روی 17946)
curl -sS -XPOST http://127.0.0.1:17946/join \
  -H 'Content-Type: application/json' \
  -d '{"id":"n2","raftAddr":"127.0.0.1:9522"}'

# افزودن n3 به RAFT
curl -sS -XPOST http://127.0.0.1:17946/join \
  -H 'Content-Type: application/json' \
  -d '{"id":"n3","raftAddr":"127.0.0.1:9523"}'
```
یا با `clusterctl`:
```
go run ./cmd/clusterctl join --id n2 --raft-addr 127.0.0.1:9522 --addr 127.0.0.1:17946
go run ./cmd/clusterctl join --id n3 --raft-addr 127.0.0.1:9523 --addr 127.0.0.1:17946
```

اکنون وضعیت کلاستر را بررسی کنید:
```
curl -sS http://127.0.0.1:17946/status | jq .
```

## TLS/mTLS (اختیاری)
- سمت سرور (مدیریت): `TLSEnable: true`, `TLSCA`, `TLSCert`, `TLSKey`
- سمت کلاینت مدیریت (برای join/leave/Status): از طریق `bootstrap.Config` و کلاینت‌های HTTP/gRPC Project
- نکته: برای HTTP، schema به‌طور خودکار به https سوئیچ می‌شود وقتی TLS فعال است

## Discovery
- Static: `DiscoveryKind: "static"` و `SeedsCSV` (CSV آدرس‌های membership)
- DNS: `DiscoveryKind: "dns"`, `DNSNamesCSV`, `DNSPort`, `DiscRefresh`
- File/ENV: `DiscoveryKind: "file"`, `FilePath`, `FileEnv`, `DiscRefresh`

## Event Bus
- دریافت رویدادها:
  - `LeaderChanged`, `ElectionStart`, `ElectionEnd`, `MemberJoin`, `MemberLeave`, `MemberFailed`
- API: `ch := cl.Subscribe(ctx)`
- Callbackها: `OnLeaderChange(info)`, `OnElectionStart()`, `OnElectionEnd(info)`

## Replication (استریم gRPC با Seq/Ack)
- ارسال: فقط روی لیدر → `cl.Publish(ctx, topic, data)`
- دریافت: نودها با Subscribe خودکار به لیدر، `HandleSync` شما را صدا می‌زنند
- پنجره backpressure: `ReplWindow` (پیش‌فرض: 1024)
- Retry بازارسال: `ReplRetry` (پیش‌فرض: 1.5s)
- پایداری بافر: `ReplBufferDir` (اختیاری)
- متریک‌ها:
  - موضوعی: `go_cluster_repl_{published_total,broadcast_total,acks_total}`, `go_cluster_repl_{seq,ack_seq,lag}`, `go_cluster_repl_subs`
  - per-node: `go_cluster_repl_{acks_per_node_total,ack_seq_per_node,lag_per_node}`

## Observability
- متریک‌ها: `GET /metrics` روی پورت مدیریت هر نود (Prometheus)
- Health: `GET /healthz`
- Tracing: `pkg/observability/tracing` (OpenTelemetry)
- مانیتورینگ بیشتر: `docs/monitoring/Monitoring.md` و داشبورد `docs/monitoring/grafana-replication.json`

## نکات تولیدی
- `DataDir` برای ذخیره پایدار RAFT حتماً تنظیم شود
- مدیریت را روی پورت/اینترفیس جدا از membership اجرا کنید (امنیت/مشاهده‌پذیری)
- TLS/mTLS را در محیط‌های غیر Dev فعال کنید
- ReplWindow/Retry را بر اساس بار تنظیم کنید؛ ReplBufferDir برای بازیابی پس از ری‌استارت توصیه می‌شود
- پورت‌های membership/raft/management را در فایروال باز کنید

## اشکال‌زدایی
- عدم تبدیل به رأی‌دهنده: اطمینان از فراخوانی `join` مدیریتی با `raftAddr` صحیح
- mismatch پروتکل مدیریت: `mgmt-proto` را روی همه نودها واضح تنظیم کنید (http یا grpc)
- lag بالا: متریک‌های lag/ack_seq را بررسی کنید، window را افزایش دهید، consumerها را بهبود دهید
- TLS: مسیر CA/Cert/Key صحیح و تطابق SNI (`TLSServerName`) را کنترل کنید

موفق باشید!
