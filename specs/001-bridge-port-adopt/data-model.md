# Phase 1 Data Model: Adopt an existing bridge port membership instead of failing

**Feature**: `001-bridge-port-adopt` | **Date**: 2026-07-30 | **Plan**: [plan.md](./plan.md)

Maps the spec's Key Entities onto concrete types. Nothing here is persisted by the provider — the device
is the system of record and Terraform state holds only the row id.

## Entity mapping

| Spec entity | Representation | Notes |
| --- | --- | --- |
| **Membership** | `MikrotikItem` (`map[string]string`) read from `/interface/bridge/port` | Keys are RouterOS kebab-case wire names. Identity is `.id`. |
| **Declaration** | `*schema.ResourceData` for `routeros_interface_bridge_port`, serialized by `TerraformResourceDataToMikrotik` (`routeros/mikrotik_serialize.go:117`) | Reused verbatim on the adoption path so the version-aware drift `transformSet` still applies. |
| **Orphaned membership** | A `Membership` whose `bridge` value resolves to nothing in `/interface/bridge` | Value may be a name or a `*`-prefixed internal id (observed `bridge=*1C`, 7.21.5). |
| **Managed set** | Terraform state — a membership is in the set iff its `.id` is stored as the resource id | Adoption inserts; declaration removal deletes via the unchanged `DefaultDelete` (FR-012). |
| **Conflict** | `*DeviceError` with `StatusCode == 400` and `Detail == "device already added as bridge port"` | See [contracts/device-rest.md](./contracts/device-rest.md). |

## Wire fields used

From `/interface/bridge/port` (only these are read for classification; the full row is handed to
`MikrotikResourceDataToTerraform` as today):

| Key | Meaning | Used for |
| --- | --- | --- |
| `.id` | Row identity, e.g. `*1` | Adoption target for `UpdateItem`; becomes the Terraform resource id |
| `interface` | The held interface, e.g. `ether5` | Lookup key — `ReadItemsFiltered(["interface=ether5"], …)` |
| `bridge` | Owning bridge, by name or internal id | Classification |
| `inactive` | Present when the row is not forwarding | Reported in diagnostics only; **never** treated as failure (spec edge case "adopted but inactive") |

From `/interface/bridge`: `.id` and `name`, for resolving whether the holding bridge still exists.

## Types

```text
DeviceError               // routeros/mikrotik_errors.go (NEW)
  Method      string      // "POST" / "GET" / "PATCH" / "DELETE" (restMethodName[method])
  URL         string      // request URL as today
  StatusCode  int         // 400
  Message     string      // "Bad Request"
  Detail      string      // "device already added as bridge port"
  Error()     string      // BYTE-IDENTICAL to routeros/mikrotik_client_rest.go:101 output

AsDeviceError(error) (*DeviceError, bool)   // errors.As wrapper

adoptPolicy                // routeros/resource_actions_adopt.go (NEW)
  adoptOff | adoptOrphaned | adoptAny        // from provider attr adopt_bridge_ports

conflictClass
  classNone      // no single unambiguous existing row → not adoptable
  classOrphaned  // holding bridge does not exist
  classSameBridge// holding bridge == declared bridge
  classMoved     // holding bridge exists and differs → requires adoptAny

adoption
  RowId       string   // .id of the adopted row
  Interface   string
  FromBridge  string   // as held on the device (may be an internal id)
  ToBridge    string   // declared
  Class       conflictClass
  Inactive    bool
```

`ExtraParams` (`routeros/mikrotik_client.go:46-48`) carries the policy from provider configuration to the
resource layer, alongside the existing `SuppressSysODelWarn` field — no new plumbing shape is
introduced. It is populated in `NewClient` (`routeros/mikrotik_client.go:50`) and reached from the
resource layer via `m.(Client).GetExtraParams()`.

## Classification state machine

Entered **only** when `CreateItem` returned a conflict `*DeviceError` **and** policy `!= adoptOff`.

```text
conflict
  │
  ├─ ReadItemsFiltered(["interface=<declared>"], "/interface/bridge/port")
  │
  ├─ 0 rows ─────────────────► classNone   → return the ORIGINAL device error, unchanged
  ├─ >1 rows ────────────────► classNone   → fail: ambiguous, device invariant broken
  └─ exactly 1 row
        │
        ├─ row.bridge == declared bridge ─► classSameBridge ─┐
        │                                                    ├─ policy >= adoptOrphaned → ADOPT
        ├─ resolve row.bridge in /interface/bridge            │
        │     (by name; then by .id if "*"-prefixed)          │
        │     └─ not found ──────────────► classOrphaned ────┘
        │
        └─ found and != declared ─────────► classMoved
                                              ├─ policy == adoptAny        → ADOPT
                                              └─ policy == adoptOrphaned   → fail: move not opted in
```

Every non-adopting exit is a failure or the original error. There is no exit that mutates the device
without a matching policy value — the fail-closed property Principle IV requires.

## Adoption sequence

```text
1. UpdateItem(&ItemId{Id, row[".id"]}, "/interface/bridge/port", declaration, client)   // PATCH, no delete
2. on error → return diag error, DO NOT SetId                                            // D5, FR-009
3. d.SetId(row[".id"])                                                                   // enters managed set
4. append one diag.Warning describing the adoption                                       // FR-010, SC-006
5. MikrotikResourceDataToTerraform(updatedRow, s, d)                                     // as ResourceCreate does
```

Step 1 satisfies FR-004 (non-destructive: no `DELETE` is issued on any path). Step 3 satisfies FR-007
and FR-008 — from this point the row is read, diffed, updated, and deleted by the existing unchanged
`DefaultRead` / `DefaultUpdate` / `DefaultDelete`, which is also what gives FR-012 for free.

## Invariants

- **INV-1**: With `adopt_bridge_ports = "off"`, no code in this feature issues a device request.
- **INV-2**: No adoption path issues `DELETE`.
- **INV-3**: A membership enters the managed set only after its convergence PATCH succeeded.
- **INV-4**: The number of emitted adoption warnings equals the number of `.id`s that entered the
  managed set via adoption (SC-006 — no silent adoptions).
- **INV-5**: An unrecognized device error is re-returned unchanged, never reinterpreted.
