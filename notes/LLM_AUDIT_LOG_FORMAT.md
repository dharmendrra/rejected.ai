# LLM Audit Log Format

The backend can record an audit trace of every LLM call (Ollama or Anthropic) to
help diagnose failures, latency, and JSON parser errors. It is **off by default**.

## Enabling

Set `LLM_LOG_LEVEL` in `config.json`:

| Value | Behavior |
|---|---|
| `"off"` (default) | No audit log is written. |
| `"info"` | One record per call with **metadata only** — model, prompt/response sizes, duration, and any error. No prompt or response bodies, so it never writes candidate text (**PII-safe**). |
| `"debug"` | Everything in `info` **plus the full system/user prompts and the raw model output**. Useful for diagnosing parse failures, but it **contains candidate data** — use deliberately. |

When enabled, the server logs a line at boot:

```
[LLM] audit logging enabled (level=info) -> logs/llm_calls.log
```

## File

- Path: `logs/llm_calls.log` (created on first run; `logs/` is gitignored).
- Format: **JSON Lines** — one JSON object per line, appended per call. The file
  is append-only across runs; rotate/delete it yourself if it grows.
- Concurrency: writes are mutex-guarded, safe for the parallel report pipeline.

## Record schema

Each line is one `auditRecord` (see `internal/llm/logging.go`):

| Field | Type | Always present? | Description |
|---|---|---|---|
| `time` | string (RFC3339Nano, UTC) | yes | When the call started. |
| `model` | string | yes | Model name (e.g. `gemma4:e4b`, `claude-sonnet-4-6`). |
| `system_len` | int | yes | Byte length of the system prompt. |
| `user_len` | int | yes | Byte length of the user prompt. |
| `output_len` | int | yes | Byte length of the model output (0 on error). |
| `duration_ms` | int | yes | Wall-clock duration of the call in milliseconds. |
| `error` | string | only on error | The error message (omitted on success). |
| `system` | string | **debug only** | Full system prompt. |
| `user` | string | **debug only** | Full user prompt. |
| `output` | string | **debug only** | Full raw model output. |

## Examples

`info` level (PII-safe), one successful call and one failure:

```json
{"time":"2026-06-08T08:30:01.123456Z","model":"gemma4:e4b","system_len":612,"user_len":1840,"output_len":742,"duration_ms":48213}
{"time":"2026-06-08T08:31:10.984210Z","model":"gemma4:e4b","system_len":612,"user_len":1840,"output_len":0,"duration_ms":1503,"error":"ollama generate status 500: model runner has stopped"}
```

`debug` level (adds bodies; truncated here for readability):

```json
{"time":"2026-06-08T08:32:00.001Z","model":"gemma4:e4b","system_len":612,"user_len":1840,"output_len":742,"duration_ms":50112,"system":"You are a seasoned technical interviewer ...","user":"Interview level: Senior Engineer | type: ...","output":"{\"questions\":[ ... ]}"}
```

## How it works

`internal/llm/logging.go` defines a `loggingCaller` decorator that wraps the
configured `Caller`. `llm.New` wraps the backend caller with it when
`LLM_LOG_LEVEL` is `info`/`debug`, so **every** call site (all engines go through
`Provider.Caller`, including `llm.CallJSON`) is logged uniformly without changing
call sites. Parser failures surface here because the raw `output` at `debug` level
is exactly what `CallJSON` tried (and failed) to parse.

## Common uses

- **Latency triage:** sort lines by `duration_ms` to find slow calls (expected to
  be large with local Ollama on CPU).
- **Parser errors:** when an engine returns "unmarshal model JSON" errors, switch
  to `debug` and inspect the `output` field for the malformed/truncated response.
- **Failure diagnosis:** filter lines with an `error` field to see backend errors
  (timeouts, non-200 status, etc.).
