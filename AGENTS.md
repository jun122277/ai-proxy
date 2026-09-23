# Collaboration boundaries

This repository is a hands-on SRE portfolio. The user explicitly owns the
important SRE design and implementation work in M1-02 and M1-03: timeout
hierarchies, cancellation/backpressure, admission limits, readiness/drain, and
metrics design. Do not implement these areas on the user's behalf unless they
explicitly ask for that work. Preparing mock fixtures, SDK contract tests,
reproduction tooling, and reviewing the user's changes is authorized.

The handoff is documented in `docs/exercises/m1-lifecycle.md`. Continue to honor
this boundary in later milestones; clarify ownership before taking over the
user's learning work.

Conserve GitHub Actions usage. Run checks locally while developing. Open draft
PRs and mark ready only once for the final commit when merging; do not dispatch
full CI unless necessary. Pushes and PR edits intentionally do not run CI.
