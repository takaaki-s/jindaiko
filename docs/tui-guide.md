# TUI Guide

## Architecture

BubbleTea (Elm Architecture) ベースの TUI。

```
tui/
├─ model.go       ... メインModel, Update(), View() (~1460行, 最大)
├─ createform.go  ... セッション作成フォーム (~500行)
├─ dirpicker.go   ... ディレクトリピッカー (~700行)
├─ styles.go      ... lipglossスタイル定義
├─ helpview.go    ... ヘルプ表示
└─ notifyview.go  ... 通知履歴表示
```

## Model構造

`model.go` の `Model` がアプリケーション全体の状態を保持:
- セッション一覧 + 選択状態
- ビューモード（リスト/作成フォーム/ヘルプ等）
- daemon.Client（IPC通信用）
- ポーリングタイマー

## Update/View パターン

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // キーバインド処理（config.GetKeybindings()参照）
    case tickMsg:
        // 定期ポーリング（daemon.Client.List()）
    case sessionListMsg:
        // セッション一覧更新
    }
}

func (m Model) View() string {
    // ビューモードに応じた描画
    // lipgloss でスタイリング
}
```

## ビューモード

- **セッション一覧**: デフォルト画面、セッションのステータス表示
- **作成フォーム**: `createform.go` の `CreateForm` モデル
- **ヘルプ**: `helpview.go` のキーバインド一覧
- **通知履歴**: `notifyview.go` の通知表示
- **ディレクトリピッカー**: `dirpicker.go` のファイルシステムブラウザ

## スタイリング

- `styles.go` で lipgloss スタイルを定義
- 生のANSIコードは使わない
- カラーは lipgloss.Color() で指定

## 新規ビュー/ポップアップ追加手順

1. 新しい `.go` ファイルを `internal/tui/` に作成
2. `tea.Model` インターフェースを実装（またはサブモデルとして組み込み）
3. `model.go` の `Model` にフィールド追加
4. `Update()` でメッセージハンドリング追加
5. `View()` でモード判定と描画追加
6. 既存の createform.go / helpview.go をパターン参考にする

## キーバインド

キーバインドは `config.GetKeybindings()` から取得される。
デフォルト値は `config.DefaultKeybindings()` で定義。
ユーザーは `~/.ccvalet/config.yaml` の `keybindings` セクションでカスタマイズ可能。
