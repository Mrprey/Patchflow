# Patchflow

Patchflow is a terminal application for running patch and release workflows in a Git repository.

It brings the most common steps of a backport or release flow into a single interface:

1. choose the base remote
2. choose the comparison ref
3. select commits
4. choose an existing branch or create a new one
5. apply cherry-picks
6. run validation and versioning steps
7. push
8. create a tag
9. generate a GitHub draft release

## Requirements

- `git`
- `gh` authenticated with `gh auth login`
- A configured editor, either in the system or in `.patchflow.yml`
- Go `1.24.2` for local development

## How to Run

The binary accepts these commands:

- `patchflow`
- `patchflow start`
- `patchflow resume`
- `patchflow config`
- `patchflow doctor`

If no command is provided, the default behavior is the same as `start`.

You can target another repository with `--repo` and another config file with `--config`:

```bash
patchflow --repo /path/to/other-repo start
patchflow --repo /path/to/other-repo --config /path/to/other-repo/.patchflow.yml doctor
```

For local development:

```bash
go test ./...
go build ./cmd/patchflow
go run ./cmd/patchflow start
```

## Overview

The main flow is driven by a Bubble Tea TUI. The real logic is not simulated: the application calls `git`, `gh`, the editor, and shell commands directly, and logs command output into `.patchflow/logs`.

By default, the application writes generated files inside the repository:

- `.patchflow/state.json` stores the current session state
- `.patchflow/logs/*.log` stores command output
- `.patchflow/release-notes.md` stores the release notes generated at the end

## Interface Flow

### 1. Welcome

When you open `start`, the welcome screen asks you to press `Enter`.

This action:

- lists local remotes
- selects the base remote
- continues to the comparison ref selection

### 2. Select Remote

This screen lets you choose the remote that will be fetched before comparing refs.

When confirmed:

- the application runs `git fetch <remote> --tags --prune`
- then loads remote refs and tags for the next step

### 3. Select Compare Base

Here you choose which remote ref will be used as the comparison base.

The list is built from:

- remote branches from the selected remote
- local tags

### 4. Select Compare Target

After selecting the compare base, the application loads the possible comparison targets.

This list includes:

- `HEAD`
- local branches
- local tags

When confirmed, the app runs a `git log` over the `base...target` range and builds the commit list.

### 5. Select Commits

This is the screen where you choose which commits will be applied.

Each commit can show:

- short SHA
- title
- PR number, when found
- PR title
- PR labels

PR enrichment is done best effort through `gh pr list`. If the lookup fails, the flow continues and the commits still appear normally.

### 6. Select Checkout Target

After selecting the commits, you choose the checkout target.

The options are:

- local branches
- local tags
- the special `Create new branch from branch/tag` option

If you choose an existing target, the application runs `git checkout <target>`.

If you choose to create a new branch, the flow continues through:

- choosing the branch/tag base
- entering the new branch name
- validating that the branch name does not already exist
- `git checkout -b <branch> <base>`

### 7. Cherry-pick Progress

After checkout, the selected commits are applied one by one with `git cherry-pick <sha>`.

If `post_cherry_pick_command` is configured, it runs after each applied commit.

If that command fails, the interface pauses on a validation screen where you can:

- open the editor
- run validation again
- continue anyway
- abort the step

If a cherry-pick stops because of a conflict, the interface switches to the conflict screen, where you can:

- continue with `git cherry-pick --continue`
- skip with `git cherry-pick --skip`
- abort with `git cherry-pick --abort`

### 8. Versioning

After cherry-picks finish, the application enters the versioning stage.

This step is used to:

- open the editor
- run the configured validation command
- continue to the push step when confirmed

### 9. Push

This step pushes the current branch to the remote configured in `default_push_remote`.

### 10. Tag

Here the application:

- checks whether the tag already exists locally
- checks whether the tag already exists on the remote
- creates an annotated tag with `git tag -a`
- pushes the tag

When empty, the default tag name is `v1.2.1`.

### 11. Release

In this step the application:

- builds release notes from the selected commits
- writes the file configured in `release_notes_file`
- creates a GitHub draft release with `gh release create --draft`

When empty, the release title uses the same value as the tag.

### 12. Done

The interactive flow is complete.

## Shortcuts

### Global

- `Enter` starts or confirms the current step
- `Esc` clears the current filter when a filter is active; otherwise it goes back to the previous screen
- while loading, `Esc` cancels the operation and preserves the current screen
- `Ctrl+C` exits the application

### Lists

- `↑` / `↓` move focus
- typing applies fuzzy filtering
- `Backspace` / `Delete` remove characters from the current filter or text field

### Commit Selection

- `Space` toggles a commit
- `Ctrl+A` selects all
- `Ctrl+D` clears all
- `Alt+A` selects only the items currently visible through the filter
- `Alt+D` clears only the items currently visible through the filter
- `Shift+↑` / `Shift+↓` select a range up to the current focus
- `Ctrl+Enter` opens the focused commit's PR in the browser, if a URL is available

### Versioning

- `e` opens the configured editor
- `r` runs the configured validation command
- `Enter` continues to the next step
- `p` jumps directly to the push step

### Push

- `Enter` runs the push
- `t` jumps to the tag step

### Tag

- `Enter` creates and pushes the tag
- `l` jumps to the release step

### Release

- `Enter` creates the draft release

### Cherry-pick Conflict

- `e` opens the editor
- `c` continues the cherry-pick
- `s` skips the current commit
- `a` aborts the cherry-pick

### Validation Failure

- `e` opens the editor
- `r` runs validation again
- `c` continues despite the failure
- `a` returns to the previous state

## `doctor`

`doctor` checks the minimum conditions required for the flow to work:

1. the directory must be a Git repository
2. the working tree must be clean
3. `gh auth status` must succeed

If something fails, the command exits with a direct error message.

## `config`

`config` prints the effective configuration, already merged with defaults.

If the file does not exist, defaults are used automatically.

### Example `.patchflow.yml`

```yaml
default_remote: upstream
default_push_remote: origin
editor: "code --wait"
post_cherry_pick_command: "flutter analyze"
release_notes_file: ".patchflow/release-notes.md"
versioning:
  suggested_files:
    - pubspec.yaml
    - CHANGELOG.md
    - android/app/build.gradle
```

## `resume`

`resume` reads `.patchflow/state.json` and prints the saved snapshot of the current session.

At the moment it does not reopen the UI exactly where it left off; it is meant to inspect the persisted flow state.

## Persisted State

The `.patchflow/state.json` file stores, among other things:

- current step
- current commit
- selected remote
- compare base and compare target
- branch being created
- tag name
- release name
- list of selected commits

This makes it possible to inspect progress even after the flow is interrupted.

## Default Configuration

If you do not provide a `.patchflow.yml`, the application uses these defaults:

- `default_remote: upstream`
- `default_push_remote: origin`
- `editor: code --wait`
- `post_cherry_pick_command: flutter analyze`
- `release_notes_file: .patchflow/release-notes.md`

## Important Notes

- Shell commands executed by the application are logged in `.patchflow/logs`.
- The flow uses fuzzy filtering on list screens.
- PR enrichment failures and `gh` errors do not stop commit loading.
- The `versioning.suggested_files` field exists in the configuration and is preserved by the parser, but the current UI does not consume it directly yet.
