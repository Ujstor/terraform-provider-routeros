# Contract: Provider surface and diagnostics

**Feature**: `001-bridge-port-adopt` | **Date**: 2026-07-30

Two contracts are specified here: the Terraform-visible provider attribute, and the exact text of every
diagnostic the feature emits. Diagnostic text is a deliverable under the constitution's
"Diagnostics are a deliverable" constraint, so it is written here and asserted in tests, not improvised
in code.

## 1. Provider attribute

Added to `Provider().Schema` in `routeros/provider.go`, beside the existing `bulk_read` /
`bulk_read_paths` / `rest_timeout` entries.

```hcl
provider "routeros" {
  # …existing arguments…
  adopt_bridge_ports = "orphaned"   # "off" (default) | "orphaned" | "any"
}
```

| Property | Value |
| --- | --- |
| Name | `adopt_bridge_ports` |
| Type | `schema.TypeString` |
| Optional | `true` |
| Default | `MultiEnvDefaultFunc([]string{"ROS_ADOPT_BRIDGE_PORTS"}, "off")` |
| Validation | `validation.StringInSlice([]string{"off", "orphaned", "any"}, false)` |
| Description | `Recovery behaviour when creating a bridge port that the device already holds. "off" (default) fails as before. "orphaned" adopts a membership whose bridge no longer exists, or one already on the declared bridge, converging it in place. "any" additionally re-points a port away from a different, live bridge — traffic-affecting. (env: ROS_ADOPT_BRIDGE_PORTS)` |

**Semantics** (normative):

| Value | Orphaned membership | Already on declared bridge | Held by a different, live bridge |
| --- | --- | --- | --- |
| `off` | fail (FR-003) | fail (FR-003) | fail (FR-003) |
| `orphaned` | adopt | adopt | fail (FR-006) |
| `any` | adopt | adopt | adopt |

**Backward compatibility**: an unset attribute with no environment variable resolves to `off`, under
which no behaviour, request sequence, or message changes (FR-011, Principle I). An invalid value fails at
provider configuration time, before any device request.

**Deliberately not added**: any resource-level attribute. Adoption is a run-time recovery policy, not
declared state; putting it in the resource schema would place it in every plan and state file and would
force a change in the consuming `tfmodule-routeros-*` modules.

## 2. Diagnostics

`{iface}`, `{bridge}`, `{holder}`, `{id}` are substituted. Tests assert on these strings.

### D-1 — Conflict, adoption off (FR-003, US3 scenario 1) — **Error**

```text
Summary: Interface {iface} is already a bridge port
Detail:  The device already holds {iface} as a port of bridge {holder}, so it cannot be added to
         {bridge}. Set adopt_bridge_ports = "any" on the provider (env ROS_ADOPT_BRIDGE_PORTS) to
         adopt the existing membership and converge it to this configuration, or remove the
         membership on the device.
```

### D-2 — Conflict caused by an orphaned membership, adoption off (US3 scenario 2) — **Error**

```text
Summary: Interface {iface} is held by a bridge port whose bridge no longer exists
Detail:  The device holds {iface} as a port of {holder}, which is not an existing bridge — the port
         row outlived its bridge and still reserves the interface. Set adopt_bridge_ports =
         "orphaned" on the provider (env ROS_ADOPT_BRIDGE_PORTS) to adopt it into {bridge}, or
         remove the port row on the device.
```

Distinguishing D-2 from D-1 is the point of FR-003's "or states that the holding bridge no longer
exists": on the reference device every one of 24 failures was this case, and the upstream 400 said
nothing about it.

### D-3 — Move not opted in (FR-006, US3 scenario 4, SC-008) — **Error**

```text
Summary: Interface {iface} is a port of live bridge {holder}
Detail:  Adopting it into {bridge} would re-point a port that is currently in service, which is
         traffic-affecting. adopt_bridge_ports = "orphaned" does not permit this. Set
         adopt_bridge_ports = "any" to allow it, or remove {iface} from {holder} on the device.
```

### D-4 — Ambiguous device state (D7) — **Error**

```text
Summary: Interface {iface} has more than one bridge port row
Detail:  The device reports {n} bridge port rows for {iface}; exactly one was expected. Adoption
         will not guess which to take over. Inspect /interface/bridge/port on the device and remove
         the duplicates.
```

### D-5 — Convergence failed after adoption (FR-009, D5) — **Error**

```text
Summary: Adopted bridge port {iface} could not be converged
Detail:  The existing membership for {iface} was found and taken over, but applying the declared
         settings failed: {err}. The membership was left unmanaged so the next apply can retry
         adoption; it may hold partially applied settings. No membership was deleted.
```

### D-6 — Adoption succeeded (FR-010, SC-006) — **Warning**, one per adoption

```text
Summary: Adopted existing bridge port {iface} into {bridge}
Detail:  Bridge port {id} for {iface} existed on the device ({class}) and was converged to this
         configuration instead of being created. Previous bridge: {holder}.
```

`{class}` is one of `orphaned bridge`, `already on this bridge`, `moved from a live bridge`. Where the
adopted row is inactive, ` The port is currently inactive.` is appended — informational only, never an
error (spec edge case "adopted but inactive").

### D-7 — Unrecognized device error — **no new diagnostic**

The original error is returned unchanged (INV-5). This is the fall-through that keeps a mis-detected
conflict from becoming a wrong message or a wrong action.

## 3. Error text stability

`DeviceError.Error()` MUST render exactly the current format string from
`routeros/mikrotik_client_rest.go:101`:

```go
"%v '%v' returned response code: %v, message: '%v', details: '%v'"
//  method, url, statusCode, message, detail
```

A test asserts this byte-for-byte. Any change to it is a Principle I violation, because users and
existing tests match on that text.
