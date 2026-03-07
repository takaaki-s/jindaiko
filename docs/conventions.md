# Coding Conventions

## Language & Formatting

- Go 1.24.5
- `make fmt` (go fmt) を必ずコミット前に実行
- コメントは日本語を継続（既存コードに合わせる）
- Technical terms, struct/function名は英語のまま

## Error Handling

- エラーは呼び出し元に return で伝播させる
- ログ出力は境界（daemon server, manager等）でのみ行う
- `fmt.Errorf("context: %w", err)` でラップする

## Debug Logging

各パッケージに `debugEnabled` / `debugLog()` が重複配置されている（共通化されていない）。

```go
var debugEnabled = os.Getenv("CCVALET_DEBUG") == "1"

func debugLog(format string, args ...interface{}) {
    // ファイルに追記: [HH:MM:SS] message
}
```

新規パッケージにデバッグログが必要な場合は、同じパターンを複製する。

## Configuration Access

- 設定値は必ず `config.Manager` 経由で取得する
- `viper` を直接呼び出してはならない（config パッケージ外では）
- `config.Manager` と `config.StateManager` は別物

## Concurrency

- `sync.RWMutex` のフィールド名は `mu`
- Lock ordering: session.Manager.mu が中心的なロック
- I/O操作（Store.Save, transcript読み取り）はロック外で実行する
  - 例: `List()` は RLock でスナップショット取得後、ロック外でtranscript読み取り

## Naming

- パッケージ名: 単数形 (`session`, `daemon`, `host`)
- JSON tags: snake_case (`json:"work_dir"`)
- Runtime-onlyフィールド: `json:"-"` タグ
- 定数: `StatusXxx` 形式 (`StatusRunning`, `StatusIdle`)

## Struct Design

- `Session`: 永続化フィールド + ランタイムフィールド（`json:"-"`）
- `Info`: 外部公開用の読み取り専用構造体（`ToInfo()` で変換）
- `Request`/`Response`: IPCメッセージ（`json.RawMessage` で型柔軟性確保）

## Testing

テストファイルは現時点で未整備。新規コードには `_test.go` ファイルを追加すること。
