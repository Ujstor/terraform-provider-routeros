<!--
Sync Impact Report
Version change: (none) → 1.0.0
Rationale: initial ratification. No prior constitution existed in this fork.
Modified principles: none (all five are new)
Added sections: Core Principles (I–V), Fork & Change Constraints,
  Development Workflow & Quality Gates, Governance
Removed sections: none
Templates requiring no change: spec-template.md, plan-template.md, tasks-template.md,
  checklist-template.md (all read the constitution at runtime)
Follow-up TODOs: none
-->

# terraform-provider-routeros (ujstor fork) Constitution

This repository is a **fork** of an upstream Terraform provider, maintained because required fixes
land in upstream `main` faster than they reach a release. Every rule below exists to keep the fork
cheap to carry and safe to point at production network devices.

## Core Principles

### I. Upstream-Compatible by Default

Default behaviour MUST match upstream. Any behavioural divergence MUST be reachable only by
explicit opt-in, and MUST default to the upstream behaviour. A user who does not opt in MUST
observe upstream semantics, including error text and resource lifecycle.

*Rationale:* the fork is consumed by pinning a commit, not by reading its changelog. Silent
divergence turns every rebase into an archaeology exercise and makes upstream bug reports
untrustworthy.

### II. Fork Debt Is a First-Class Cost

Every divergence from upstream MUST be justified in a spec, MUST be as small as the problem allows,
and SHOULD be shaped so it can be contributed upstream unchanged. Divergences MUST NOT be spread
across files when they can be localized. Repository-local artifacts (specs, tooling) MUST be
separable from source changes so a single commit can be cherry-picked upstream.

*Rationale:* this fork exists to be temporary. Anything that makes it permanent is a defect.

### III. Behaviour Changes Ship With Failing-First Tests (NON-NEGOTIABLE)

A change to provider behaviour MUST land with at least one test that fails against the previous code
and passes against the new code, demonstrated in that order. Where a defect was caused by a class of
mistake, the tests MUST include a guard for the class, not only the instance.

*Rationale:* a dead version-mapping entry shipped in this codebase in a form that could never match,
was reviewed, and was released — because nothing asserted that the entry actually fired. An
instance-only test would not have caught it either.

### IV. Device Safety Over Convenience

The provider MUST NOT take a destructive or traffic-affecting action on a device implicitly. When
declared intent conflicts with device state, the default MUST be to fail loudly rather than mutate.
Every failure an operator is expected to act on MUST name the object, name the conflicting state,
and name the remediation — the device's raw rejection text alone is NOT an acceptable diagnostic.

*Rationale:* these devices carry production traffic and, frequently, the only management path to
themselves. A wall of identical upstream 400s cost hours of diagnosis that one actionable sentence
would have prevented.

### V. Version-Aware Device Contracts Are Explicit and Verified

Where device behaviour differs across RouterOS versions, the difference MUST be expressed as an
explicit, tested mapping — never as an untested assumption or a silent fallback. Every such mapping
MUST record the version it was verified against. A mapping that cannot match any real input is a
defect regardless of whether anything currently fails.

*Rationale:* RouterOS renames wire properties between releases. Handling that is this provider's
core job, so a mapping that silently does nothing is a total failure of the primary function.

## Fork & Change Constraints

- **Generated code is generated.** Files marked as generated MUST be produced by their generator,
  never hand-edited. A change to generated output MUST be made at its source and regenerated.
- **No dead configuration.** A configuration entry that cannot affect behaviour MUST NOT be added,
  and MUST be treated as a defect when found.
- **Consumers pin immutable refs.** This fork is consumed by commit SHA, not by branch. Changes MUST
  assume consumers upgrade deliberately, and MUST NOT rely on a moving ref to deliver a fix.
- **Diagnostics are a deliverable.** Error paths that operators hit MUST be specified and tested with
  the same rigour as success paths.
- **Existing suites stay green.** `go build ./...` and the offline test suite MUST pass. Tests
  requiring live device credentials are exempt from the green requirement but MUST NOT regress in
  count or coverage.

## Development Workflow & Quality Gates

- **Spec-Driven Development is mandatory** for every behaviour change. Every feature flows
  constitution → spec → plan → tasks → implement. Technology choices live in the plan, never in the
  spec.
- **No implementation code before `spec.md` exists and its quality gate passes.** Open
  `[NEEDS CLARIFICATION]` markers block `plan.md`.
- **The Constitution Check in `plan.md` is a gate**, not a formality. An unjustified violation stops
  the plan.
- **Trivial changes are exempt**: dependency bumps, typos, and comment-only edits do not require the
  pipeline. A change to what the provider does on a device is never trivial.
- **Verify against a real device or a fixture, and say which.** Claims about device behaviour MUST
  state how they were confirmed and on what version.

## Governance

This constitution supersedes conflicting local practice. It governs this fork only and makes no claim
on upstream.

- **Amendments** require a documented rationale, a version bump per the policy below, and a Sync
  Impact Report recorded in this file.
- **Versioning policy:** MAJOR for backward-incompatible governance changes or principle removal;
  MINOR for a new principle or materially expanded guidance; PATCH for clarifications and wording.
- **Compliance review:** every spec's plan MUST include a Constitution Check. `analyze` treats a
  violation as CRITICAL. Reviewers MUST verify Principle I (opt-in default) and Principle III
  (failing-first test) explicitly, because those two are what a rushed fix skips.

**Version**: 1.0.0 | **Ratified**: 2026-07-30 | **Last Amended**: 2026-07-30
