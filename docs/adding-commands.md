# Adding CLI Commands

## 手順

### 1. コマンドファイル作成

`cmd/ccvalet/cmd/` に新規 `.go` ファイルを作成。

### 2. Cobra コマンド定義

```go
package cmd

import "github.com/spf13/cobra"

var myCmd = &cobra.Command{
    Use:   "my-command",
    Short: "Short description",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 実装
        return nil
    },
}

func init() {
    // トップレベルコマンドの場合:
    rootCmd.AddCommand(myCmd)

    // sessionサブコマンドの場合:
    // sessionCmd.AddCommand(myCmd)
}
```

### 3. デーモン通信が必要な場合

```go
import "github.com/takaaki-s/claude-code-valet/internal/daemon"

client, err := daemon.NewClient()
if err != nil {
    return fmt.Errorf("daemon not running: %w", err)
}
defer client.Close()

// IPC呼び出し
resp, err := client.Send(daemon.Request{
    Action: "my-action",
    Data:   data,
})
```

### 4. セッション名の解決

セッション名/ID の曖昧な指定を解決するには、既存の `resolveSessionName()` を使用する。

### 5. コマンド階層

```
ccvalet (root)
├─ daemon start/stop/status
├─ session
│   ├─ new
│   ├─ list
│   ├─ kill
│   ├─ attach
│   └─ ...
├─ tui
├─ hook
└─ (新規コマンドはここに追加)
```

## 参考ファイル

- シンプルなコマンド: `kill.go`, `list.go`
- フラグ付きコマンド: `new.go`
- サブコマンドグループ: `session.go`
