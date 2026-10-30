# Design Doc: Worker Task Executors

**Branch:** `feature/worker-executors` (to be created)
**Status:** Planning

## Goal

Abhi worker sirf hardcoded `time.Sleep(2 * time.Second)` karta hai, task ke
`type`/`payload` ko dekhta hi nahi. Is feature me worker ko **real task
types** execute karna sikhana hai, jaisa `jobflow-design-doc.md` me
planned tha (`type TEXT NOT NULL, -- http, shell, sleep (demo)`).

## Task types aur examples

### 1. `http` — external API call

Use case: kisi service ko trigger karna (data extract, webhook, notify).

```json
{
  "name": "extract-sales-data",
  "type": "http",
  "payload": {
    "method": "POST",
    "url": "https://api.mycompany.com/extract",
    "body": { "date": "2026-10-30" }
  }
}
```

Worker: `url` pe HTTP request bhejega, `Idempotency-Key` header me
`task.idempotency_key` bhejega (taaki target service retry pe duplicate
detect kar sake), response status code check karega (`2xx` = success).

### 2. `shell` — local command/script

Use case: server pe koi script chalana (cleanup, backup, local processing).

```json
{
  "name": "cleanup-temp-files",
  "type": "shell",
  "payload": {
    "command": "find /tmp/jobflow -mtime +1 -delete"
  }
}
```

Worker: `os/exec` se command run karega. Exit code `0` = success, warna
`stderr` ko `ReportResult.error` me daal ke failure report karega.

### 3. `sql` — database query/maintenance job

Use case: scheduled reports, materialized view refresh, cleanup queries
(bahut common job-scheduler use case, jaisa Airflow/cron-based ETL me).

```json
{
  "name": "refresh-daily-summary",
  "type": "sql",
  "payload": {
    "query": "REFRESH MATERIALIZED VIEW daily_sales_summary;"
  }
}
```

Worker: apne Postgres connection pool (ya target DB ke liye alag
connection string) se ye query chalayega, error aaye to failure report
karega.

### (demo type, already exists) `sleep`

Testing/demo ke liye — payload me `seconds` diya jaata hai:
```json
{ "name": "demo-wait", "type": "sleep", "payload": { "seconds": 5 } }
```

## Plan (high level)

1. `Task.Payload` (`[]byte`, JSON) ko type ke hisaab se alag struct me
   unmarshal karna (`SleepPayload`, `HTTPPayload`, `ShellPayload`, `SQLPayload`)
2. Har type ka apna executor function (`executors/` package, jaisa
   `jobflow-design-doc.md` me `worker/executors/` planned tha):
   - `executors.RunSleep(payload)`
   - `executors.RunHTTP(payload, idempotencyKey)`
   - `executors.RunShell(payload)`
   - `executors.RunSQL(ctx, pool, payload)`
3. Worker ka main loop: `task.Type` ke switch se sahi executor call kare,
   result (`error`) se `Success`/`Error` field decide kare `ReportResult` me
4. Tests: har executor ke liye chhota unit test (success + failure case)

## Open questions

- `sql` executor ke liye konsa DB connection use hoga — scheduler ka hi
  pool, ya worker apna alag connection banayega? (abhi discuss karna hai)
- `shell` executor security risk hai (arbitrary command execution) —
  design doc me already "non-goal: arbitrary code sandboxing" likha hai,
  isliye demo/trusted-payload assumption ke saath chalenge abhi
