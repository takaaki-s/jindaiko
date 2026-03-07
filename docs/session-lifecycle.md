# Session Lifecycle

## Status State Machine

```
                    CreateWithOptions()
                          │
                          ▼
                     StatusStopped
                          │
                   StartBackground()
                          │
                          ▼
                    StatusRunning ◄─── RecoverTmuxSessions()
                     │    │    │
    UserPromptSubmit │    │    │ Notification(permission_prompt)
                     ▼    │    ▼
              StatusThinking  StatusPermission
                     │    │    │
                Stop │    │    │ Stop
                     ▼    ▼    ▼
                    StatusIdle
                          │
              pane dead / Kill()
                          │
                          ▼
                    StatusStopped
```

Status定数 (session/session.go):
- `creating`   - CC起動中（現在未使用、予約）
- `stopped`    - プロセス停止
- `running`    - 実行中（Hook未受信の初期状態）
- `idle`       - 入力待ち（Stop hook）
- `thinking`   - 処理中（UserPromptSubmit hook）
- `permission` - 許可待ち（Notification hook）

## Session Structure

```go
Session (永続化)
├─ ID              string    // UUID (Claude Code --session-id互換)
├─ Name            string    // 表示名 (デフォルト: WorkDirのbasename)
├─ WorkDir         string    // 作業ディレクトリ（hookのcwdで動的更新）
├─ Status          Status
├─ ClaudeSessionID string    // Claude Code セッションID
├─ ClaudeSessionStarted bool // --resume vs --session-id の判定用
├─ TmuxWindowName  string    // inner tmuxセッション名
├─ TmuxPaneID      string    // CC pane ID (例: "%42")
├─ HostID          string    // "local" or リモートホスト名
└─ LastActiveAt    time.Time

Session (ランタイムのみ, json:"-")
├─ LastOutputTime  time.Time // idle安定性検出用
├─ StartedAt      time.Time // 起動直後のエラー誤検出防止
├─ SSHAuthSock    string    // git操作用
├─ CurrentWorkDir string    // tmux pane_current_path
├─ CurrentBranch  string    // git branch
└─ IsGitRepo      bool
```

## Creation Flow

1. `Manager.CreateWithOptions()` で Session 生成 + Store 永続化
2. `Manager.StartBackground()` → `startSession()` → `startSessionTmux()`
3. `ensureTmuxClient()` で inner tmux (`-L ccvalet`) を初期化
4. `ensureClaudeTrustState()` で `~/.claude/settings.local.json` のtrust設定
5. inner tmux session を作成、`claude --session-id {ID}` を実行
6. `TagManagedPane()` で remain-on-exit 対象タグ付与
7. `captureOutputTmux()` goroutine でポーリング開始

## Recovery (Daemon再起動時)

`RecoverTmuxSessions()`:
1. 全永続化セッションをロード（Status=Stopped で初期化）
2. TmuxWindowName があるセッションについて inner tmux 生存確認
3. 生存 → StatusRunning + `captureOutputTmux()` 再開
4. pane dead → StatusStopped（TmuxWindowName は保持、RespawnPane 用）
5. session自体消失 → TmuxWindowName クリア + StatusStopped

## Resume失敗時の自動リカバリ

`captureOutputTmux()` 内で起動10秒以内のpane死亡を検出:
1. `claude --resume` が失敗したと判断
2. 新しいClaudeSessionIDを生成
3. `claude --session-id {新ID}` で RespawnPane
4. 成功すれば新セッションとして継続

## WorkDir追従

WorkDirは2つの経路で更新される:
1. **Hook経由**: `HandleHookEvent()` の `cwd` フィールド（Claude Codeの実CWD）
2. **Polling経由**: `captureOutputTmux()` の `GetPaneCurrentPath()`（tmux paneのCWD）
