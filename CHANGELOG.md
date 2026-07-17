# Changelog

goreleaser assembles per-release notes from Conventional Commits history and
attaches them to the corresponding [GitHub Release](https://github.com/takaaki-s/jind-ai/releases).
This file is the curated overview — highlights per release, not a per-commit
log.

## 0.8.0

### Features

- **A plugin can now declare multiple actions.** Manifests support an
  `actions:` list where each entry carries its own `id` / `entrypoint` /
  `on` / `popup` / `label`. The first entry is the implicit default action
  (no explicit flag). Palette rows, keybindings, and `jin plugin run` all
  operate at the action level. Existing v1 manifests (`schema_version: 1`
  with top-level `entrypoint` / `on`) are normalised at parse time into a
  single-action shape, so plugin authors need to do nothing to keep
  working.
- **`jin plugin run <name> [action]`** — an optional second positional
  argument selects the action. Omitted invocations run the default action
  (`actions[0]`) and keep the exact pre-existing output
  (`Started plugin <name> (global)`) so scripts that grep the CLI's
  success message stay working. Shell completion is two-stage: plugin
  names first, then that plugin's action IDs.
- **`JIN_ACTION_ID`** env var — every plugin run receives the ID of the
  action that fired it, so a shared entrypoint script can dispatch on
  `$JIN_ACTION_ID` instead of parsing argv.
- **`jin plugin install` / `update` consent screens are per-action.** v2
  manifests now render every action's `id` / `entrypoint` / `on`. This
  also fixes a v1-era rendering path that read the (now-forbidden)
  top-level `on:` field and left the `Events:` line empty for v2 plugins.
- **`jin plugin validate --run-build` checks every action's entrypoint**
  exists after build, not just the default action's — so multi-binary
  v2 plugins get the full sanity check.

### Breaking changes

- **`keybindings.plugins.<name>.keys`** → **`keybindings.plugins.<name>.actions.<id>.keys`**.
  The old shape is detected at load time, a single WARN is logged per
  plugin, and that plugin's binding is dropped (TUI startup is never
  blocked). Rewrite by hand; for a plugin with only a default action,
  `actions.default.keys: [...]` is the shortest translation.

  ```yaml
  # Before (0.7.x — ignored with a WARN under 0.8.0)
  keybindings:
    plugins:
      notifier:
        keys: ["M-n"]

  # After (0.8.0)
  keybindings:
    plugins:
      notifier:
        actions:
          default:
            keys: ["M-n"]
          send-dm:
            keys: ["M-d"]
  ```

- **`plugin.EventDispatcher.RunAction`** signature is now
  `RunAction(name, actionID, ev, depth, actx)`. An empty `actionID` selects
  the default action; an unknown ID returns a synchronous error listing
  the available actions.
- **`daemon.PluginRunRequest`** gains an `Action string` field (empty =
  default action). IPC wire-compat is preserved: pre-0.8.0 clients simply
  omit the field and land on the default action, matching their previous
  behaviour.
- **`plugin.PopupSizeResolver`** signature is now
  `(pluginName, actionID string, m *manifest.PopupConfig) → (w, h string)`.
  The daemon's built-in resolver ignores `actionID` for now — per-action
  popup size in user config is out of scope for this release — but the
  argument is plumbed so a later config schema can widen without another
  breaking signature change.

## 0.7.2

### Bug Fixes

- **`jin plugin install <name>` / `jin plugin update` now clone from
  `github.com`.** Registry entries record `repo` as bare `owner/name`
  (the crawler's GitHub `FullName`), but the resolver was passing that
  string to `git clone` unchanged and hitting an unresolvable host.
  Bare entries are now prefixed with `https://github.com/` before
  clone; entries that already carry a URL scheme (mirrors, `file://`
  fixtures) pass through untouched.

## 0.7.1

### Features

- **`install.source.build` is now optional.** Plugins that ship a directly
  executable entrypoint (shell scripts, prebuilt binaries checked into the
  repo) can omit the `build` block entirely and point `install.source.entrypoint`
  at the script or binary. Only `install.source.entrypoint` remains required
  under `install.source`. Existing manifests that declare a `build` block
  continue to validate unchanged.

## 0.7.0

### Features

- **Plugin registry** — new `jin plugin ls-remote`, `jin plugin install <name>`
  (registry-resolved with SHA pin and a consent screen), and `jin plugin
  validate` commands. See [docs/plugin-registry.md](docs/plugin-registry.md)
  for the discover/install/publish flow and full flag reference.
- **Unified plugin manifest (`jind-ai-plugin.yaml`)** — the runtime dispatcher
  and the registry crawler now read the same file with the same schema. The
  old `jin-plugin.yaml` / `api_version` shape has been removed.
- **`pkg/plugin/manifest`** — the manifest package is now exported so the
  registry crawler and any third-party tool validate manifests bit-for-bit
  identically to jin itself.

### Breaking changes

`0.7.0` is a pre-1.0 minor bump and carries breaking changes to the plugin
system. See [docs/plugin-registry.md#pre-10-break-policy](docs/plugin-registry.md#pre-10-break-policy)
for the policy in full.

- The plugin manifest file is now `jind-ai-plugin.yaml` (was
  `jin-plugin.yaml`); the `api_version` field is gone and `schema_version: 1`
  takes its place. Existing plugins must migrate the file name, add
  `schema_version` / `name` / `version` / `description` / `jin:`, and move
  `run` / `build` under `install.source.{entrypoint,build[]}`.
- The built-in desktop notifier has been removed from the daemon. Install
  [`jind-ai-notifier`](https://github.com/takaaki-s/jind-ai-notifier) — the
  same notifier repackaged as a plugin — to restore the behaviour.
