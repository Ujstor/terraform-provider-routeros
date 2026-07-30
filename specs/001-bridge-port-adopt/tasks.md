# Tasks: Adopt an existing bridge port membership instead of failing

**Feature**: `001-bridge-port-adopt` | **Date**: 2026-07-30

**Input**: [spec.md](./spec.md), [plan.md](./plan.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: mandatory here. Constitution Principle III is non-negotiable, and the ordered
failing-then-passing pair is an explicit task (T017 → T018 → T019).

**Format**: `- [ ] T### [P] [US#] Description with exact/file/path.ext`
`[P]` = no shared file with another `[P]` task in the same phase, safe to run in parallel.

**Paths**: all source paths are relative to the repository root; the provider package is flat in
`routeros/`.

---

## Phase 1: Setup

- [x] T001 Record the pre-change baseline: run `go build ./...` and `go test ./routeros/...`, and save the summary for the commit body — the constitution's "existing suites stay green" gate compares against this. **Captured at `b62c425`**: build clean; 18 PASS, 1 FAIL (`TestClientTransport_SendRequest`, needs `ROS_HOSTURL`/`ROS_USERNAME`), then the binary aborts in `Test_mikrotikResourceDataToTerraform` via `log.Fatal` at `routeros/mikrotik_resource_drift_implementation.go:68-71`. Details in [research.md](./research.md) §5.

**Checkpoint**: baseline captured. The suite is **not** green at base and this feature does not make it
green — the abort is pre-existing and out of scope (research §5). Because the abort kills the test
binary, every task below MUST verify with a `-run` filter; a bare `go test ./routeros/...` never reaches
`resource_actions_adopt_test.go`.

---

## Phase 2: Foundational — blocking prerequisites

These close the two gaps identified in [research.md](./research.md) §1: the resource layer cannot see
*why* a create failed, and nothing here is testable offline. No user story can start until they are done.

- [ ] T002 [P] Create `routeros/mikrotik_errors.go` with `DeviceError{Method, URL, StatusCode, Message, Detail}`, an `Error()` that renders the format string from `routeros/mikrotik_client_rest.go:101` verbatim, and `AsDeviceError(error) (*DeviceError, bool)` wrapping `errors.As`.
- [ ] T003 [P] Create `routeros/mikrotik_errors_test.go` asserting `Error()` is byte-identical to the current `fmt.Errorf` output for a representative 400, and that `AsDeviceError` round-trips through `fmt.Errorf("%w", …)` wrapping — per [contracts/provider-schema.md](./contracts/provider-schema.md) §3.
- [ ] T004 Return `*DeviceError` instead of the bare `fmt.Errorf` in `routeros/mikrotik_client_rest.go:93-103`, leaving the rendered message unchanged. Depends on T002.
- [ ] T005 [P] Add the `adopt_bridge_ports` attribute to `Provider().Schema` in `routeros/provider.go` beside `bulk_read`/`bulk_read_paths` (~:104-129), with `MultiEnvDefaultFunc(["ROS_ADOPT_BRIDGE_PORTS"], "off")` and `validation.StringInSlice(["off","orphaned","any"], false)`, exactly as specified in [contracts/provider-schema.md](./contracts/provider-schema.md) §1.
- [ ] T006 Add `AdoptBridgePorts` to `ExtraParams` in `routeros/mikrotik_client.go:46-48` and populate it at **both** construction sites in `NewClient` — the API client at `routeros/mikrotik_client.go:134` and the REST client at `:172` — from `d.Get("adopt_bridge_ports")`. Editing only one leaves the other transport with a zero-valued policy. Depends on T005.
- [ ] T007 Create `routeros/resource_actions_adopt.go` with the `adoptPolicy` type (`adoptOff`/`adoptOrphaned`/`adoptAny`), a parser from the attribute string, and `policyFromMeta(m interface{})` reading it via `GetExtraParams()`. Depends on T006.
- [ ] T008 Add `isBridgePortConflict(*DeviceError) bool` to `routeros/resource_actions_adopt.go` with the detail string from [contracts/device-rest.md](./contracts/device-rest.md) R-1 as a single named constant carrying its `verified: RouterOS 7.21.5` comment. Depends on T002.
- [ ] T009 Create `routeros/resource_actions_adopt_test.go` with a table test for `isBridgePortConflict` covering: the exact 7.21.5 detail, case/whitespace variants, a 400 with an unrelated detail, a non-400 `DeviceError`, and a plain `error` — Principle V requires proof the matcher actually fires. Depends on T008.
- [ ] T010 [P] Add a table test for the provider attribute in `routeros/provider_test.go`: `Provider().InternalValidate()` still passes, the default resolves to `off` with no env set, `ROS_ADOPT_BRIDGE_PORTS` is honoured, and an invalid value is rejected. Depends on T005.

**Checkpoint**: conflicts are detectable as data, the policy is configurable and validated, and nothing
about provider behaviour has changed yet.

---

## Phase 3: User Story 1 — Onboard a device whose interfaces are already bridge ports (P1)

**Goal**: an apply against a device holding pre-existing memberships for the declared interfaces
succeeds in one run with no manual device changes (SC-001, SC-002, SC-004, SC-005).

**Independent test**: `go test ./routeros/... -run TestAdoptOnConflict` — the stub device starts with an
orphaned row for the declared interface, and the create must succeed by adopting it.

### Test harness (offline)

- [ ] T011 [US1] Add a stub `Client` to `routeros/resource_actions_adopt_test.go` implementing `GetExtraParams`, `GetTransport` (REST) and `SendRequest`, which records every `(method, url, item)` in order and replays scripted responses including a 400 conflict `*DeviceError`. First such stub in the repo — keep it confined to this file (research D8).
- [ ] T012 [P] [US1] Add a helper to the same file that builds a `*schema.ResourceData` for `routeros_interface_bridge_port` from a declaration map, using `ResourceInterfaceBridgePort().Schema` so the real schema and serializer are exercised.
- [ ] T013 [P] [US1] Add fixtures to the same file for the R-1 conflict response, the orphaned-row read (`bridge=*1C`, `inactive=true`), the same-bridge row, the live-bridge row, the empty bridge lookup and the successful PATCH — transcribed from [contracts/device-rest.md](./contracts/device-rest.md).

### Failing-first evidence (Principle III — ordering is the deliverable)

- [ ] T014 [US1] Write `TestAdoptOnConflict_OrphanedRowIsAdopted` in `routeros/resource_actions_adopt_test.go`: policy `orphaned`, stub returns the R-1 conflict on create then the orphaned row, and the test asserts the create returns no error, the resource id equals the existing row's `.id`, and a PATCH was issued to that id. Depends on T011–T013.
- [ ] T015 [US1] Run `go test ./routeros/... -run TestAdoptOnConflict_OrphanedRowIsAdopted` against the still-unwired code and record the failure output in the commit body / PR description. This is the "fails first" half of the constitution's non-negotiable pair (SC-007) — it must be captured before T016.

### Implementation

- [ ] T016 [US1] Implement `locateMembership` and `classifyConflict` in `routeros/resource_actions_adopt.go` per the state machine in [data-model.md](./data-model.md): filtered read on `interface=`, 0 rows → return the original error, >1 rows → ambiguous failure, else resolve the held bridge by `name` then by `.id` when `*`-prefixed, yielding `classOrphaned` / `classSameBridge` / `classMoved`. Depends on T007, T008.
- [ ] T017 [US1] Implement `AdoptOnConflictCreate(s map[string]*schema.Schema) schema.CreateContextFunc` in `routeros/resource_actions_adopt.go`: delegate to the unmodified `ResourceCreate`; on error, only if `AsDeviceError` + `isBridgePortConflict` + policy `!= adoptOff` do the recovery — classify, check the policy against the class, `UpdateItem` the declaration onto the existing `.id`, and only then `d.SetId` (research D5, INV-3). `routeros/resource_actions.go` must not be edited. Depends on T016.
- [ ] T018 [US1] Switch `CreateContext` from `DefaultCreate(resSchema)` to `AdoptOnConflictCreate(resSchema)` at `routeros/resource_interface_bridge_port.go:448`. One line; this is the only change to the resource. Depends on T017.
- [ ] T019 [US1] Re-run `go test ./routeros/... -run TestAdoptOnConflict_OrphanedRowIsAdopted` and record the pass. Completes the failing-first pair with T015. Depends on T018.

### Remaining US1 coverage

- [ ] T020 [P] [US1] `TestAdoptOnConflict_SameBridgeRowIsAdopted` in `routeros/resource_actions_adopt_test.go` — a row already on the declared bridge is adopted under `orphaned` (FR-005).
- [ ] T021 [P] [US1] `TestAdopt_NeverIssuesDelete` in the same file — assert across the orphaned, same-bridge and `any`-move paths that the recorded request log contains no `DELETE` (FR-004, INV-2). This is the class guard, not an instance test.
- [ ] T022 [P] [US1] `TestAdopt_MixedDeclarationsSingleRun` in the same file — two declarations where one conflicts and one does not: the first is adopted, the second created, both in one pass (US1 scenario 3).
- [ ] T023 [P] [US1] `TestAdopt_NoConflictIsUntouched` in the same file — policy `orphaned` with a create that succeeds issues exactly the requests today's code issues, no extra read (FR-014, contract R-5).
- [ ] T024 [P] [US1] `TestAdopt_AdoptedRowIsReadBack` in the same file — after adoption the PATCH response is fed through `MikrotikResourceDataToTerraform` so state matches the device, which is what makes the next plan empty (SC-002) and a single edited setting produce exactly one change (SC-004).
- [ ] T025 [US1] Verify no serialization divergence: assert in `routeros/resource_actions_adopt_test.go` that the PATCH body on the adoption path equals the PUT body the create path would have sent, so the drift `transformSet` (`routeros/mikrotik_serialize.go:206-213`) still applies (contract R-4).

**Checkpoint**: US1 is complete and independently valuable — the blocking onboarding failure is gone.
Everything below is safety, recovery and messaging on top of a working feature.

---

## Phase 4: User Story 2 — Re-adopt a configured device after losing tracking state (P2)

**Goal**: a device configured by an earlier successful run, with state that no longer references it,
reconciles on apply instead of failing per membership.

**Independent test**: `go test ./routeros/... -run TestReadopt` — same mechanism as US1 with the
"existing row already matches the declaration" shape.

No new production code is expected; US2 is US1's mechanism under a different starting state, and these
tests exist to prove that claim rather than assume it.

- [ ] T026 [P] [US2] `TestReadopt_MatchingRowReportsNoDeviceChange` in `routeros/resource_actions_adopt_test.go` — the existing row already equals the declaration: it is adopted, the PATCH is a no-op in effect, and the resulting state matches the device (US2 scenario 1).
- [ ] T027 [P] [US2] `TestReadopt_DivergedRowIsCorrected` in the same file — the existing row differs in one setting: adoption converges it to the declared value (US2 scenario 2).
- [ ] T028 [US2] If T026/T027 reveal that US2 needs production code beyond T016/T017, implement the minimum in `routeros/resource_actions_adopt.go` and note it in [research.md](./research.md) §4 as a resolved unknown. Otherwise close this task with "no code required — covered by T017".

**Checkpoint**: state recovery is a routine apply.

---

## Phase 5: User Story 3 — Keep conflicts loud on devices that are not ours to change (P3)

**Goal**: the default stays fail-closed, and every failure names the object, the conflicting state and
the remediation (FR-003, FR-006, Principle IV). Diagnostic text comes verbatim from
[contracts/provider-schema.md](./contracts/provider-schema.md) §2.

**Independent test**: `go test ./routeros/... -run 'TestAdoptDisabled|TestAdoptMove|TestAdoptDiagnostic'`.

- [ ] T029 [US3] Implement diagnostics D-1 (conflict, adoption off), D-2 (orphaned holder, adoption off), D-3 (move not opted in), D-4 (ambiguous, >1 row) and D-5 (convergence failed after adoption) in `routeros/resource_actions_adopt.go`, using the exact wording from the contract. Depends on T017.
- [ ] T030 [US3] Implement the D-6 adoption warning in `routeros/resource_actions_adopt.go` — one `diag.Warning` per adoption naming interface, bridge, class and row id, with the inactive suffix when the row reports `inactive` (FR-010, SC-006, INV-4). Depends on T017.
- [ ] T031 [P] [US3] `TestAdoptDisabled_RequestSequenceUnchanged` in `routeros/resource_actions_adopt_test.go` — with policy `off` and a conflicting device, the recorded request log is exactly today's (one create, nothing else — INV-1) and the returned error is the unchanged `DeviceError` text (FR-011, SC-003, Principle I). The guard against future leakage into the default path.
- [ ] T032 [P] [US3] `TestAdoptDiagnostic_OrphanHolderIsNamed` in the same file — with policy `off` and an orphaned holder the emitted diagnostic is D-2, not D-1 and not the raw 400 (US3 scenario 2). Also assert exactly one diagnostic per conflicting interface across a multi-interface case, and that it names the interface and its holder (SC-003). Depends on T029.
- [ ] T033 [P] [US3] `TestAdoptMove_RequiresAnyPolicy` in the same file — a row held by a live, existing bridge under policy `orphaned` fails with D-3 and **no** PATCH is recorded; under `any` the same case is adopted (SC-008, FR-006). Depends on T029.
- [ ] T034 [P] [US3] `TestAdoptAmbiguous_TwoRowsFail` in the same file — two rows for the declared interface produce D-4 and no mutation (research D7). Depends on T029.
- [ ] T035 [P] [US3] `TestAdoptConvergeFailure_LeavesNoState` in the same file — a failing PATCH after a successful classification returns D-5, leaves the resource id empty, and records no `DELETE` (FR-009, D5, INV-3). Depends on T029.
- [ ] T036 [P] [US3] `TestAdoptUnknownDeviceError_PassesThrough` in the same file — a 400 with an unrelated detail and a non-`DeviceError` failure are both returned unchanged under every policy value (INV-5, research D7).

**Checkpoint**: all three user stories complete; the dangerous path is gated and every failure is
actionable.

---

## Phase 6: Polish & gates

- [ ] T037 Run `go build ./...`, then `go test ./routeros/...` and a `-run` pass covering the new tests, and confirm **no new failures against the T001 baseline** — the two pre-existing failures documented in [research.md](./research.md) §5 remain, and this feature neither fixes nor worsens them (SC-007, constitution: existing suites stay green).
- [ ] T038 [P] Run `gofmt -l routeros/` and `go vet ./routeros/...` clean over the touched files.
- [ ] T039 [P] Document `adopt_bridge_ports` in the provider docs alongside `bulk_read` — the generated/authored docs page for the provider configuration — matching the contract wording so the shipped description and the spec cannot drift.
- [ ] T040 [P] Add a short note to `README.md` (or the fork's divergence notes, wherever `bulk_read`/`rest_timeout` are recorded) listing this as a fork divergence with its default-off guarantee, so the next rebase can find it (Principle II).
- [ ] T041 Verify against a real device: with `ROS_ADOPT_BRIDGE_PORTS=orphaned`, apply the N-DC1-NBG1-SW1 configuration to a device holding orphaned rows, confirm one run and zero manual changes (SC-001), then re-apply and confirm zero changes (SC-002). Record the RouterOS version in the PR body per the constitution's "say which" rule.
- [ ] T042 Optional, does not block: prepare the upstream contribution shape — confirm the source diff is separable from `specs/` and `.specify/`, and that T002+T004 stand alone as a typed-error commit (Principle II).

---

## Dependencies

```text
T001
 └─► Phase 2
      T002 ─┬─► T003
            ├─► T004
            └─► T008 ─► T009
      T005 ─┬─► T006 ─► T007 ─► T008
            └─► T010

Phase 2 complete
 └─► Phase 3 (US1)
      T011 ─┬─► T014 ─► T015 (evidence: MUST fail) ─► T016 ─► T017 ─► T018 ─► T019 (evidence: passes)
      T012 ─┤
      T013 ─┘
      T017/T018 ─► T020, T021, T022, T023, T024, T025

Phase 3 complete (US1 shippable)
 ├─► Phase 4 (US2): T026, T027 ─► T028
 └─► Phase 5 (US3): T029, T030 ─► T031…T036

Phases 3–5 complete
 └─► Phase 6: T037 ─► T038, T039, T040, T041, T042
```

**Hard ordering that must not be reordered**: T014 → T015 → T016/T017/T018 → T019. Writing the
implementation before capturing T015's failure forfeits the Principle III evidence, and the constitution
names this as the gate a rushed fix skips.

## Parallel opportunities

- Phase 2: T002+T005+T010 concurrently; then T003+T004 once T002 lands.
- Phase 3 harness: T012 and T013 alongside T011's stub (different concerns, same file — sequence the
  writes if one agent owns the file).
- Phase 3 coverage: T020–T024 are independent tests once T018 lands.
- Phase 5: T031–T036 all independent once T029/T030 land.
- Phase 6: T038, T039, T040 concurrently.

Note that most `[P]` tests in Phases 3–5 land in the same file
(`routeros/resource_actions_adopt_test.go`); they are independent in content but not in file, so a
single writer should batch them.

## Implementation strategy

**MVP is Phase 1 + Phase 2 + Phase 3.** That alone clears the blocking onboarding failure and can ship
on its own. Phase 4 is proof-of-claim with little or no new code. Phase 5 is where the constitution's
device-safety principle is actually paid for — do not defer it past a release, because `orphaned` without
D-3 would mean the move case is reachable with a misleading message.

**Requirement coverage**: FR-001 T017; FR-002 T005; FR-003 T029/T032; FR-004 T021; FR-005 T020;
FR-006 T033; FR-007 T024; FR-008 T024/T026; FR-009 T035; FR-010 T030; FR-011 T031; FR-012 unchanged
`DefaultDelete` + T018 (verified by T037's suite); FR-013 unchanged device validation + T036;
FR-014 T023.

**Success-criteria coverage**: SC-001 T041; SC-002 T024/T026/T041; SC-003 T032; SC-004 T024;
SC-005 T041; SC-006 T030; SC-007 T015/T019/T037; SC-008 T033.
