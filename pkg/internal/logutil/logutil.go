package logutil

import (
    "encoding/json"
    "fmt"
    "log"
    "os"
    "sync/atomic"
    "time"
)

var jsonMode atomic.Bool

func init() {
    if os.Getenv("CLUSTER_LOG_JSON") == "1" || os.Getenv("CLUSTER_LOG_FORMAT") == "json" {
        jsonMode.Store(true)
    }
}

func prefix(l *log.Logger, p string) *log.Logger {
    if l == nil { l = log.Default() }
    return log.New(l.Writer(), p, l.Flags())
}

func SetJSON(enabled bool) { jsonMode.Store(enabled) }

func Infof(l *log.Logger, f string, args ...any)  { logf(l, "info", f, args...) }
func Warnf(l *log.Logger, f string, args ...any)  { logf(l, "warn", f, args...) }
func Errorf(l *log.Logger, f string, args ...any) { logf(l, "error", f, args...) }

func logf(l *log.Logger, level, f string, args ...any) {
    if jsonMode.Load() {
        // emit structured json
        msg := fmt.Sprintf(f, args...)
        evt := map[string]any{
            "ts":    time.Now().UTC().Format(time.RFC3339Nano),
            "level": level,
            "msg":   msg,
        }
        b, _ := json.Marshal(evt)
        if l == nil { l = log.Default() }
        l.Println(string(b))
        return
    }
    switch level {
    case "info":
        prefix(l, "INFO ").Printf(f, args...)
    case "warn":
        prefix(l, "WARN ").Printf(f, args...)
    default:
        prefix(l, "ERROR ").Printf(f, args...)
    }
}
