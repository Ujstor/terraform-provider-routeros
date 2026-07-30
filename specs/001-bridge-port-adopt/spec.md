# Feature Specification: Adopt an existing bridge port membership instead of failing

**Feature Branch**: `as/bridge-port-adopt-on-conflict`

**Created**: 2026-07-30

**Status**: Draft — awaiting review

**Input**: User description: "Adopt an existing bridge port membership instead of failing when the device already holds that interface as a bridge port, gated behind an opt-in that defaults off"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Onboard a device whose interfaces are already bridge ports (Priority: P1)

An operator applies a switch configuration to a device where some or all of the declared interfaces
are already recorded as bridge ports — because of factory default configuration, a previous partial
apply, or prior service in another role. With adoption enabled, the apply brings those memberships
under management, converges them to the declared state, and reports success. No manual device
changes are required beforehand.

**Why this priority**: this is the blocking failure. It has cost multiple failed runs and manual
device surgery on a single switch, and it recurs on every device that has ever been powered on with
default configuration. Nothing else in this feature matters if this does not work.

**Independent Test**: point a configuration at a device that already holds memberships for the
declared interfaces, apply once with adoption enabled, and assert the run succeeds and the device
matches the declaration. Delivers the entire value of the feature on its own.

**Acceptance Scenarios**:

1. **Given** a device where each declared interface is held by an orphaned membership, **When** the
   operator applies with adoption enabled, **Then** the run succeeds and every interface is a port
   of the declared bridge carrying the declared settings.
2. **Given** the same device immediately after that run, **When** the operator applies again with no
   configuration change, **Then** the run reports no changes.
3. **Given** a device where only some declared interfaces conflict, **When** the operator applies,
   **Then** the conflicting ones are adopted and the rest are created normally, in a single run.
4. **Given** an adopted membership, **When** the operator later changes one declared setting for it,
   **Then** the next run plans and applies exactly that change.

---

### User Story 2 - Re-adopt a configured device after losing tracking state (Priority: P2)

Tracking state is lost, rebuilt, or relocated while the device remains fully configured. The
operator applies and the run reconciles against what is already on the device rather than failing on
every membership.

**Why this priority**: same root cause as US1, and it turns state recovery from a manual rebuild into
a routine apply. Lower than P1 only because it happens far less often than onboarding.

**Independent Test**: apply successfully, discard the tracking state, apply again with adoption
enabled, and assert the second run succeeds and reports the device already matches.

**Acceptance Scenarios**:

1. **Given** a device configured by an earlier successful run and tracking state that no longer
   references it, **When** the operator applies with adoption enabled, **Then** every declared
   membership is adopted and no device change is made beyond re-establishing tracking.
2. **Given** that situation but with one declared setting since changed on the device by hand,
   **When** the operator applies, **Then** the divergence appears in the planned changes and is
   corrected to match the declaration.

---

### User Story 3 - Keep conflicts loud on devices that are not ours to change (Priority: P3)

An operator applies to a device that is shared or already in production service. Adoption is off, so
a conflict fails the run rather than silently re-pointing an interface out of whatever bridge it
currently serves. The failure states which interface conflicts, what holds it, and how to proceed.

**Why this priority**: adoption is destructive by implication — it re-points live ports. Default-off
with an actionable diagnostic preserves today's safety while US1 and US2 opt in. Ranked P3 only
because it is mostly the *absence* of new behaviour plus better messaging.

**Independent Test**: apply to a device with a conflicting membership, leaving adoption at its
default, and assert the run fails with a diagnostic naming the interface and its current holder.

**Acceptance Scenarios**:

1. **Given** a conflicting membership and adoption at its default, **When** the operator applies,
   **Then** the run fails and the diagnostic names the interface, names the bridge currently holding
   it, and states that adoption can be enabled to converge it.
2. **Given** a conflict caused by an orphaned membership, **When** the operator applies with adoption
   at its default, **Then** the diagnostic states that the interface is held by a membership whose
   bridge no longer exists, rather than only relaying the device's rejection.
3. **Given** adoption at its default and no conflicts, **When** the operator applies, **Then**
   behaviour is identical to the current release.
4. **Given** adoption enabled and a membership held by a different, existing bridge, **When** the
   operator applies without the separate move opt-in, **Then** the run fails and the diagnostic
   distinguishes this traffic-affecting case from an orphan adoption.

---

### Edge Cases

- **Orphaned membership.** A membership outlives deletion of its bridge and keeps reserving the
  interface. Observed on RouterOS 7.21.5: 24 memberships referencing a bridge that no longer
  existed, each still blocking creation. This is the common case, not an exotic one.
- **Interface ineligible.** The interface is a bond slave or otherwise cannot be a bridge port.
  Adoption must not mask a genuinely invalid declaration.
- **Two declarations, one interface.** The configuration claims the same interface twice. An
  authoring error that must surface as one regardless of the adoption setting.
- **Adoption succeeds, convergence fails.** The membership is brought under management but applying
  the declared settings then fails, leaving a managed object that does not match its declaration.
  The run must fail rather than report success.
- **Concurrent modification.** Someone edits the device by hand mid-run. Adoption must not overwrite
  in a way that loses the change without it appearing as divergence on the next run.
- **Adopted but inactive.** A membership can be adopted and correct yet inactive because its
  interface is administratively disabled or the bridge's filtering rejects it. Inactive is not
  failure and must not be reported as one.
- **Membership held by a live bridge.** Re-pointing a port that is currently forwarding is
  traffic-affecting, and is deliberately excluded from the default (see Assumptions).
- **Nothing to adopt.** Adoption is enabled but no conflict exists. Behaviour must be identical to
  adoption being off.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST converge the device to the declared state within a single run, without
  operator intervention, when a declaration conflicts with an existing membership and adoption is
  enabled for that conflict class.
- **FR-002**: Adoption MUST be opt-in and MUST default to disabled.
- **FR-003**: System MUST fail, when adoption is disabled and a conflict occurs, with a diagnostic
  that identifies the interface, identifies the bridge currently holding it or states that the
  holding bridge no longer exists, and names the remediation.
- **FR-004**: Adoption MUST be non-destructive: the existing membership MUST NOT be deleted and
  recreated in order to bring it under management.
- **FR-005**: System MUST adopt orphaned memberships through the same mechanism as memberships that
  already reference the declared bridge.
- **FR-006**: System MUST treat re-pointing a membership away from a different, existing bridge as a
  separate, additionally-gated action, and MUST NOT perform it under the base adoption opt-in alone.
- **FR-007**: After adoption, System MUST track the membership such that later divergence between
  declaration and device is detected and reported as a change.
- **FR-008**: Repeated application with an unchanged declaration MUST report no changes, whether the
  membership was created or adopted.
- **FR-009**: System MUST fail the run if convergence fails after a membership has been adopted, and
  MUST NOT report success for an object that does not match its declaration.
- **FR-010**: System MUST record each adoption in the run output, one entry per adopted membership,
  identifying the interface and the bridge it was adopted into.
- **FR-011**: System MUST leave all existing create, read, update, and delete behaviour unchanged
  when adoption is disabled.
- **FR-012**: An adopted membership MUST follow the same removal lifecycle as a created one, so that
  removing the declaration removes the membership from the device.
- **FR-013**: System MUST fail a declaration that is invalid on its own terms — an ineligible
  interface, or the same interface declared twice — regardless of the adoption setting.
- **FR-014**: Adoption MUST behave identically whether or not a conflict exists, from the operator's
  point of view, when the declared state is already satisfied.

### Key Entities

- **Membership**: the device-side record that a given interface is a port of a given bridge. Holds
  the interface, the owning bridge, and the port's settings. An interface may appear in at most one
  membership.
- **Declaration**: the operator's desired membership as expressed in configuration — which interface
  belongs to which bridge, with which settings.
- **Orphaned membership**: a membership whose owning bridge no longer exists. Still reserves its
  interface, and carries no settings worth preserving.
- **Managed set**: the memberships this configuration is responsible for. Adoption moves an existing
  membership into this set; removal of a declaration takes it out.
- **Conflict**: the state in which a declaration cannot be satisfied by creating a new membership
  because the interface is already reserved by an existing one.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Applying the reference out-of-band switch configuration to a device holding 23
  pre-existing memberships for the declared interfaces completes successfully in **one** run with
  **zero** manual device changes beforehand.
- **SC-002**: An immediate second application of the same configuration reports **zero** changes.
- **SC-003**: With adoption disabled, that same scenario fails and emits **one** diagnostic per
  conflicting interface naming the interface and its current holder; the device's raw rejection text
  is not the only information the operator receives.
- **SC-004**: After adoption, changing exactly one declared port setting produces a plan containing
  exactly one change.
- **SC-005**: Human device interactions required between "configuration merged" and "run green", for
  a device that has ever run default configuration, drop from **at least one** to **zero**.
- **SC-006**: The number of adoptions reported in the run log equals the number of pre-existing
  memberships that were adopted, with no silent adoptions.
- **SC-007**: The existing offline test suite passes unchanged, and the new behaviour is covered by
  at least one test that fails against the current code and passes after the change.
- **SC-008**: Enabling adoption on a device with a membership held by a live, existing bridge does
  **not** move that port unless the separate move opt-in is also set — verified by an explicit test.

## Assumptions

- **Destroy semantics (decision).** An adopted membership follows the same removal lifecycle as a
  created one: removing the declaration removes the membership from the device (FR-012). Chosen for a
  uniform lifecycle with no special cases, which is what a Terraform user expects. The alternative —
  releasing tracking and leaving the membership in place — was rejected as surprising and as a source
  of permanent drift. Worth confirming in review, as it is the one decision that changes destroy
  behaviour.
- **Adoption safety boundary (decision).** The base opt-in adopts orphaned memberships and
  memberships that already reference the declared bridge. Re-pointing a membership away from a
  different, existing bridge is traffic-affecting and requires an additional explicit opt-in
  (FR-006). This follows the constitution's device-safety principle: the common, safe case is easy
  and the dangerous case stays deliberate.
- **Feature scope (decision).** This feature covers bridge port membership only. Other object classes
  that reject creation because an equivalent object exists are out of scope; the design should leave a
  clean seam for reuse but MUST NOT change other resources. This keeps fork divergence minimal.
- **Delivery vehicle.** The fork is where this ships. Contributing it upstream later is desirable but
  is not a requirement of this feature, and no upstream acceptance is assumed.
- **Device reachability.** Devices remain reachable over their management API for the duration of a
  run, and the management path does not traverse the objects being changed. On the reference device
  management arrives over a routed interface that is not a bridge member, which is what makes bridge
  changes safe to apply remotely.
- **Device behaviour.** RouterOS retains memberships when their bridge is deleted and keeps enforcing
  one-bridge-per-interface against those retained records. Confirmed on 7.21.5; assumed to hold
  across 7.x but not verified on other versions.
- **Orphan contents.** An orphaned membership carries no configuration worth preserving, so adopting
  it and overwriting its settings from the declaration loses nothing of value.

## Out of Scope

- Removing or reconciling factory default configuration in general — default addresses, DHCP client,
  firewall rules. A preflight/bootstrap capability in the deployment pipeline covers that; this
  feature only makes membership declarations converge.
- Establishing a device's initial management reachability.
- Any change to how bridges, bonds, or interfaces themselves are created.
- Applying the adopt-on-conflict pattern to any other object class.
