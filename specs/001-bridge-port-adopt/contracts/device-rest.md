# Contract: RouterOS REST interactions on the adoption path

**Feature**: `001-bridge-port-adopt` | **Date**: 2026-07-30

**Verified against**: RouterOS **7.21.5** (LTS) on `N-DC1-NBG1-SW1`, REST API on port 8244.
Not verified on other releases — per constitution Principle V, every claim below carries that
qualification, and the implementation falls through to the unchanged upstream error whenever a response
does not match this contract.

Transport scope: **REST only.** The API transport (`routeros/mikrotik_client_api.go`) is not converted
to typed errors in this feature, so adoption does not engage there and behaviour is unchanged.

## R-1 — Create conflict (the trigger)

**Request** — issued today by `CreateItem` (`routeros/mikrotik_crud.go:18-40`), unchanged:

```http
PUT /rest/interface/bridge/port
Content-Type: application/json

{"bridge":"br1","interface":"ether5", …}
```

**Response** (conflict):

```http
400 Bad Request

{"error":400,"message":"Bad Request","detail":"device already added as bridge port"}
```

**Contract used**: `StatusCode == 400` **and** `Detail == "device already added as bridge port"`
(compared case-insensitively after trimming). Both must hold. A 400 with any other detail is **not** a
conflict and is returned unchanged.

**Stability risk**: this detail string is RouterOS's, not ours. If a future release rewords it, adoption
stops engaging and the operator sees today's error — degraded, never wrong. The string is defined in one
place with this version tag and is covered by a table test so it cannot rot into a mapping that never
fires.

## R-2 — Locate the existing membership

**Request** — `ReadItemsFiltered([]string{"interface=ether5"}, "/interface/bridge/port", c)`
(`routeros/mikrotik_crud.go:74-93`); bypasses the bulk-read cache, so it reads live device truth:

```http
GET /rest/interface/bridge/port?interface=ether5
```

**Response shapes and their handling**:

| Response | Class | Action |
| --- | --- | --- |
| `[]` | `classNone` | return the original R-1 error unchanged |
| exactly one object | classify per R-3 | continue |
| two or more objects | `classNone` | fail with D-4 (ambiguous) |

**Observed orphaned row** (7.21.5, after the bridge was deleted):

```json
[{".id":"*1C","interface":"ether5","bridge":"*1C","inactive":"true","pvid":"1"}]
```

Note the `bridge` value is a `*`-prefixed internal id, not a name — the referenced bridge is gone.
`inactive` is present and is informational only.

## R-3 — Resolve whether the holding bridge exists

Skipped when the row's `bridge` already equals the declared bridge (that is `classSameBridge`).

**By name**:

```http
GET /rest/interface/bridge?name=br0
```

**Then by id**, only if the held value is `*`-prefixed and the name lookup returned `[]`:

```http
GET /rest/interface/bridge?.id=*1C
```

| Outcome | Class |
| --- | --- |
| both lookups return `[]` | `classOrphaned` |
| a bridge is returned and its name differs from the declared bridge | `classMoved` |
| a bridge is returned and its name equals the declared bridge | `classSameBridge` |

**Device behaviour relied upon (7.21.5)**: deleting a bridge leaves its port rows in place, and those
rows keep enforcing one-bridge-per-interface. Observed directly — 24 rows survived bridge deletion and
each still produced the R-1 conflict. This is the premise of the whole feature; if a future RouterOS
cascades the delete, the orphan class simply stops occurring and nothing breaks.

## R-4 — Converge in place (the adoption)

**Request** — `UpdateItem(&ItemId{Id, "*1C"}, "/interface/bridge/port", declaration, c)`
(`routeros/mikrotik_crud.go:95-117`). REST appends the id to the path:

```http
PATCH /rest/interface/bridge/port/*1C
Content-Type: application/json

{"bridge":"br0","interface":"ether5", …full serialized declaration…}
```

**Response**: `200` with the updated row, which is handed to `MikrotikResourceDataToTerraform` exactly
as `ResourceCreate` does with a created row.

**Contract points**:

- The payload is the output of `TerraformResourceDataToMikrotik` — identical to what create would have
  sent — so the version-aware drift `transformSet` (`routeros/mikrotik_serialize.go:206-213`) applies
  unchanged on this path.
- `UpdateItem` invalidates the bulk-read cache for the path and id (`routeros/mikrotik_crud.go:113`), so
  a subsequent read cannot serve a stale pre-adoption row.
- **No `DELETE` is issued on any adoption path** (FR-004, INV-2). Asserted by test.
- A non-2xx response here produces D-5 and leaves the resource id unset (D5, FR-009).

## R-5 — Request budget

| Situation | Extra requests vs. today |
| --- | --- |
| `adopt_bridge_ports = "off"` | **0** — no code from this feature runs |
| Adoption on, no conflict | **0** — the recovery path is entered only on the R-1 error |
| Conflict, `classSameBridge` | 2 (R-2, R-4) |
| Conflict, `classOrphaned` | 3–4 (R-2, R-3 by name, optional R-3 by id, R-4) |
| Conflict, not adoptable | 2–3 (R-2, optional R-3), then fail |
