# Napkin Runbook

## Curation Rules
- Re-prioritize on every read.
- Keep recurring, high-value notes only.
- Max 10 items per category.
- Each item includes date + "Do instead".

## Execution & Validation (Highest Priority)
1. **[2026-06-04] TDD before implementation**
   Do instead: write failing test, then smallest code to pass, then refactor and rerun relevant tests.
1. **[2026-06-04] Verify environment blockers first**
   Do instead: if toolchain missing, confirm with a direct check and report blocker instead of guessing.

## Shell & Command Reliability
1. **[2026-06-04] Prefer isolated command runners**
   Do instead: wrap git/gh calls behind an interface so tests can stub them cleanly.

## Domain Behavior Guardrails
1. **[2026-06-04] Preserve user workflow order**
   Do instead: keep compare, select, cherry-pick, version, push, tag, release as explicit steps.
1. **[2026-06-04] PR lookup best effort**
   Do instead: treat missing PR metadata as non-blocking and keep commit flow moving.
1. **[2026-06-04] Loading stays on current screen until result**
   Do instead: show spinner/message as overlay, keep screen stable, and let ESC cancel loading without losing context.

## User Directives
1. **[2026-06-04] TDD required**
   Do instead: always start with red test, then implement, then simplify before final handoff.
