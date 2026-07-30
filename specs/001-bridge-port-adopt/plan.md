# Implementation Plan: Adopt an existing bridge port membership instead of failing

**Branch**: `as/bridge-port-adopt-on-conflict` | **Date**: 2026-07-30 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/001-bridge-port-adopt/spec.md`

## Summary

When a bridge port declaration conflicts with a membership the device already holds, the provider today
relays RouterOS's `400 … device already added as bridge port` and stops (`routeros/resource_actions.go:65-68`).
On any device that has ever run factory-default configuration this fails on every port, and it survives
bridge deletion because RouterOS keeps orphaned port rows.

The approach is a **create-recovery path scoped to one resource**: the create request is issued exactly
as today; only when it fails with the bridge-port conflict *and* adoption is opted in does the provider
look up the existing row, classify it, converge it in place with a PATCH, and take ownership of it. Two
prerequisites make that possible — a typed transport error so the resource layer can tell *why* a create
failed, and a stub `Client` so all of it is testable offline. The opt-in is one provider-level tri-state
(`off` / `orphaned` / `any`) so the traffic-affecting case (re-pointing a port away from a live bridge)
is a separate, deliberate choice, and so the deployment pipeline can enable it via an environment
variable without touching any consuming Terraform module.

Full decision record with rejected alternatives: [research.md](./research.md).

## Technical Context

**Language/Version**: Go (module `github.com/terraform-routeros/terraform-provider-routeros`; toolchain per `go.mod`)

**Primary Dependencies**: `hashicorp/terraform-plugin-sdk/v2` (`schema`, `diag`, `helper/validation`); no new dependency

**Storage**: N/A — device is the system of record; Terraform state holds the row id

**Testing**: `go test ./routeros/... -run …` (a filter is mandatory — the suite aborts early at base, research §5). Offline unit tests against a new stub `Client`; the existing `resource_*_test.go` acceptance tests need a live device and are exempt from the green gate but must not regress

**Target Platform**: Terraform provider plugin (Linux/amd64 in CI, baked into the `nuvotex-router` OCI image); devices are RouterOS 7.21.5 over the REST transport on port 8244

**Project Type**: Single Go library — a Terraform provider fork

**Performance Goals**: no additional device request on any path where adoption is disabled or no conflict occurs; adoption itself costs at most 3 extra requests per conflicting membership (filtered port read, bridge resolution, PATCH)

**Constraints**: default behaviour byte-identical to upstream including error text; no delete/recreate of an existing membership; changes localized so a single commit is cherry-pickable upstream; REST transport only (API transport documented as uncovered)

**Scale/Scope**: 2 new source files, 2 new test files, 5 touched files (4 source + `provider_test.go`); reference device holds 23–24 pre-existing memberships in a single run

## Constitution Check

*GATE: must pass before Phase 0 research. Re-checked after Phase 1 design — result below is the
post-design re-check.*

| Principle | Verdict | Evidence |
| --- | --- | --- |
| **I. Upstream-Compatible by Default** | **PASS** | `adopt_bridge_ports` defaults to `off` (D4). With it off, the executed path is today's `ResourceCreate` unchanged (D1). The typed error's `Error()` is byte-identical to the current message (D3), so even error *text* does not diverge. FR-002, FR-011. Guarded by a test asserting the exact request sequence in the default case (D8). |
| **II. Fork Debt Is a First-Class Cost** | **PASS** | Divergence is one new file (`resource_actions_adopt.go`) + one new file's worth of tests, a typed error in the REST transport, one provider attribute, and a one-line `CreateContext` change. `ResourceCreate` is not modified (D2). Each of the three pieces is independently upstreamable; `specs/` and `.specify/` are separable from source changes. |
| **III. Behaviour Changes Ship With Failing-First Tests** | **PASS** | T014 is written against pre-change code, T015 captures its failure before any wiring lands, T019 records the pass; the ordering is a hard dependency in `tasks.md`. Class guards beyond the instance: T031 asserts the request sequence is unchanged when adoption is off, T021 asserts no `DELETE` is ever issued on any adoption path, and T009 asserts the conflict matcher actually fires. SC-007. |
| **IV. Device Safety Over Convenience** | **PASS** | Default is fail-loud (FR-003). Ambiguity fails closed: 0 rows returns the original error, >1 rows fails (D7). The traffic-affecting move needs a second opt-in (`any`) with a negative test (SC-008). Failed convergence leaves no state rather than a tainted resource whose remedy would destroy a row we never owned (D5). Diagnostics name interface + holder + remediation, not the raw 400. |
| **V. Version-Aware Device Contracts Are Explicit and Verified** | **PASS** | The conflict detail string, the orphaned-row `bridge=*1C` behaviour, and the request/response shapes are recorded in [contracts/device-rest.md](./contracts/device-rest.md), each tagged `verified: RouterOS 7.21.5`. The matcher is exercised by a table test, so it cannot become a mapping that never fires. Unrecognized details fall through to the unchanged error — no silent fallback that acts. |

**Fork & Change Constraints**: no generated file is hand-edited (none is touched). No dead
configuration — T010, T031 and T033 together prove each of the attribute's three values changes
behaviour, so the switch cannot ship inert the way the drift entry did.
Consumers keep pinning a SHA. Diagnostics are specified in
[contracts/provider-schema.md](./contracts/provider-schema.md) and tested. `go build ./...` and
`go test ./routeros/...` must stay green.

**Result: PASS — no violations, no justifications required.** Complexity Tracking is empty by
consequence.

## Project Structure

### Documentation (this feature)

```text
specs/001-bridge-port-adopt/
├── spec.md                       # WHAT & WHY (approved)
├── plan.md                       # This file
├── research.md                   # Phase 0: findings + decisions D1–D8
├── data-model.md                 # Phase 1: entities → Go types, classification state machine
├── quickstart.md                 # Phase 1: enable it, reproduce it, test it
├── contracts/
│   ├── provider-schema.md        # Provider attribute + exact diagnostic texts
│   └── device-rest.md            # RouterOS request/response contract, version-tagged
├── checklists/
│   └── requirements.md           # Spec quality gate (16/16 READY)
└── tasks.md                      # Phase 2 (/speckit-tasks): T001–T042, dependency-ordered
```

### Source Code (repository root)

```text
routeros/
├── resource_actions_adopt.go        # NEW — AdoptOnConflictCreate, classification, adoption
├── resource_actions_adopt_test.go   # NEW — stub Client, failing-first + guard tests
├── resource_interface_bridge_port.go# CHANGED — one line: CreateContext (:448)
├── provider.go                      # CHANGED — adopt_bridge_ports attribute (near :104-129)
├── mikrotik_client.go               # CHANGED — ExtraParams field + NewClient wiring (:46-50)
├── mikrotik_client_rest.go          # CHANGED — return *DeviceError instead of fmt.Errorf (:93-103)
├── mikrotik_errors.go               # NEW — DeviceError type + AsDeviceError helper
├── resource_actions.go              # UNCHANGED — shared ResourceCreate stays as-is
└── mikrotik_crud.go                 # UNCHANGED — CreateItem/ReadItemsFiltered/UpdateItem reused
```

**Structure Decision**: the fork is a flat single-package Go provider (`routeros/`), one file per
resource plus shared `mikrotik_*.go` / `resource_actions*.go` helpers. This feature follows that
convention rather than introducing a subpackage: adoption lives beside the other shared create wrappers
as `resource_actions_adopt.go`, matching `resource_actions_default.go` and
`resource_actions_default_system.go`. Keeping it in-package is also what makes the change
cherry-pickable upstream (Principle II); a new subpackage would force exported-API decisions this
feature does not need.

## Phase 1 design outputs

- **[data-model.md](./data-model.md)** — the five spec entities mapped to concrete Go types and
  `MikrotikItem` keys, plus the conflict-classification state machine and its fail-closed transitions.
- **[contracts/provider-schema.md](./contracts/provider-schema.md)** — the provider attribute contract
  (type, values, default, env var, validation) and the verbatim text of every diagnostic the feature
  emits, so the messages are reviewable and testable as deliverables.
- **[contracts/device-rest.md](./contracts/device-rest.md)** — every device interaction on the adoption
  path with observed payloads, tagged with the verified RouterOS version.
- **[quickstart.md](./quickstart.md)** — how an operator enables adoption, what the run output looks
  like, how to reproduce the conflict, and how to run the offline suite.

## Complexity Tracking

No Constitution Check violations. Nothing to justify.
