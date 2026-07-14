# Changelog

goreleaser assembles per-release notes from Conventional Commits history and
attaches them to the corresponding [GitHub Release](https://github.com/takaaki-s/jind-ai/releases).
This file is the curated overview — highlights per release, not a per-commit
log.

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
