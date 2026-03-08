# Coding Conventions

## Language & Formatting

- Go 1.24.5
- Always run `make fmt` (go fmt) before committing
- Comments should be in English
- Technical terms, struct/function names remain in English

## Error Handling

- Propagate errors to the caller via return
- Log only at boundaries (daemon server, manager, etc.)
- Wrap with `fmt.Errorf("context: %w", err)`

## Debug Logging

debugEnabled / debugLog() are duplicated in the daemon and session packages (not shared).

```go
var debugEnabled = os.Getenv("CCVALET_DEBUG") == "1"

func debugLog(format string, args ...interface{}) {
    // Append to file: [HH:MM:SS] message
}
```

If a new package needs debug logging, duplicate the same pattern.

## Configuration Access

- Always access settings through `config.Manager`
- Do not call `viper` directly (outside the config package)
- `config.Manager` and `config.StateManager` are separate instances

## Concurrency

- `sync.RWMutex` field name is `mu`
- Lock ordering: session.Manager.mu is the central lock
- Perform I/O operations (Store.Save, transcript reads) outside the lock
  - Example: `List()` takes a snapshot under RLock, then reads transcripts after releasing the lock

## Naming

- Package names: singular (`session`, `daemon`, `host`)
- JSON tags: snake_case (`json:"work_dir"`)
- Runtime-only fields: `json:"-"` tag
- Constants: `StatusXxx` format (`StatusRunning`, `StatusIdle`)

## Struct Design

- `Session`: Persisted fields + runtime fields (`json:"-"`)
- `Info`: Read-only struct for external use (converted via `ToInfo()`)
- `Request`/`Response`: IPC messages (type flexibility via `json.RawMessage`)

## Testing

Coverage ~40%. Test files exist for all packages.
Uses only the standard library (no testify, etc.). Same-package tests (`package X`) allow testing unexported functions.
The `tmux.Runner` interface was introduced for testability.
Add `_test.go` files for new code.
