# Gotchas

エージェントが陥りやすい落とし穴と注意事項。

## tmux

- **remain-on-exit はペインレベル**で設定する（グローバルではない）。
  `TagManagedPane()` で管理対象ペインにのみ適用される。
  ユーザーが追加したペインは即座に破棄される。
  (commit 980e99f で修正)

- **tmux session名** は `tmux.SessionName` 定数（"ccvalet"）。変更してはならない。

- **inner tmux**: ccvaletは独自のtmuxソケット (`-L ccvalet`) を使用する。
  ユーザーのメインtmuxとは別のサーバープロセス。

- **base-index問題**: ユーザーの `~/.tmux.conf` で `base-index=1` が設定されている場合、
  `:0.0` ターゲットが無効になる。pane ID (`%N`) を使用すること。

## Hook

- **セッション識別は `CCVALET_SESSION_ID` 環境変数**（最も信頼性が高い）。
  Claude Code の session ID はフォールバック。
  (commit a0bd6f7 で改善)

- **CWD追従は hookの `cwd` フィールド**で行う。
  tmux の `pane_current_path` もポーリングで取得するが、hookが優先。
  (commit a705a80 で追加)

## コード構造

- **debugLog/debugEnabled は各パッケージに重複**している（共通化されていない）。
  新規パッケージでも同じパターンを複製する。

- **config.Manager と config.StateManager は別**のインスタンス。混同しない。

- **Session.WorkDir は動的に変更される**（hookやtmuxポーリングで更新）。
  初期値 = 作成時のディレクトリだが、claudeがcdすると追従する。

## テスト

- **テストファイルが未整備**。リファクタリング時は手動テストが必要。
  新規コードにはテストを書くこと。

## 排他制御

- **セッション作成は `createMu` で排他制御**される（daemon.Server レベル）。
  `session.Manager.mu` とは別のロック。

- **I/O操作はロック外で実行**する（deadlock防止）。
  `List()` のパターンを参照: RLock でスナップショット → ロック解除 → transcript読み取り
