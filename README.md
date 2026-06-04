# Patchflow

Patchflow is a terminal app for patch and release workflows.

## Commands

- `patchflow`
- `patchflow start`
- `patchflow resume`
- `patchflow config`
- `patchflow doctor`

## Config

Create `.patchflow.yml` in repo root.

Example:

```yaml
default_remote: upstream
default_push_remote: origin
editor: "code --wait"
post_cherry_pick_command: "flutter analyze"
release_notes_file: ".patchflow/release-notes.md"
```

## Status

Core flow, config, doctor, state, release notes, and command wrappers are scaffolded.
