# Phase 0 Research: Adopt an existing bridge port membership instead of failing

**Feature**: `001-bridge-port-adopt` | **Date**: 2026-07-30 | **Spec**: [spec.md](./spec.md)

All code references are to this repository at branch base `b62c425` (fork of upstream `development`).
All device claims are qualified with the RouterOS version they were observed on, per constitution
Principle V.

## 1. Codebase findings (what exists today)

| Question | Answer | Anchor |
| --- | --- | --- |
| How is the bridge port resource wired? | `MetaResourcePath: "/interface/bridge/port"`, `MetaId: PropId(Id)`, CRUD delegated to the shared `Default*` wrappers, plus a custom importer. | `routeros/resource_interface_bridge_port.go:84-87`, `:448-454` |
| What runs on create? | `DefaultCreate(s)` → `ResourceCreate(ctx, s, d, m)`: serialize, `CreateItem`, on error `diag.FromErr(err)` and stop. | `routeros/resource_actions_default.go:12`, `routeros/resource_actions.go:61-68` |
| Is create shared with every other resource? | Yes. `ResourceCreate` is the single create implementation for ~all resources. Changing it changes every resource. | `routeros/resource_actions.go:61` |
| Device I/O primitives available | `CreateItem`, `ReadItems`, `ReadItemsFiltered`, `UpdateItem`, `DeleteItem` — all take `(…, resourcePath, c Client)`. | `routeros/mikrotik_crud.go:18,42,74,95,119` |
| Does a filtered read bypass the bulk-read cache? | Yes. `ReadItems` consults the cache; `ReadItemsFiltered` always hits the device. | `routeros/mikrotik_crud.go:43-46` vs `:74-93` |
| How are device errors represented? | Untyped `fmt.Errorf` string built in the REST transport. No status code or detail is recoverable by a caller. | `routeros/mikrotik_client_rest.go:93-103` |
| What is the client seam? | `Client` is a 3-method interface (`GetExtraParams`, `GetTransport`, `SendRequest`). Only `RestClient` and `ApiClient` implement it. | `routeros/mikrotik_client.go:20-24`, `mikrotik_client_rest.go:58`, `mikrotik_client_api.go:50` |
| Is there an existing offline fake client? | **No.** 191 `_test.go` files; the `resource_*_test.go` ones are acceptance tests needing a live device. Offline tests (`mikrotik_cache_test.go`, `mikrotik_serialize_test.go`) test pure functions only. | `routeros/mikrotik_cache_test.go:1-40` |
| Precedent for a provider-level behaviour switch? | Yes, in this fork: `rest_timeout`, `bulk_read`, `bulk_read_paths`, with `MultiEnvDefaultFunc` env defaults and `ValidateFunc`. `helper/validation` is already imported, so `StringInSlice` needs no import change. | `routeros/provider.go:104-128`, `:5` |
| Where is `ExtraParams` built? | **Two** sites, not one — the API client and the REST client are constructed separately in `NewClient`. Both must be updated or the policy is silently zero on one transport. | `routeros/mikrotik_client.go:134`, `:172` |
| How does a filter become a URL? | `URL.Query` is joined with `&` and prefixed with `?`, so `ReadItemsFiltered([]string{".id=*1C"}, "/interface/bridge", c)` yields `/interface/bridge?.id=*1C` — the R-3 id lookup is expressible. | `routeros/mikrotik_client.go:216-222` |
| How does serialization reach the wire? | `TerraformResourceDataToMikrotik` snake→kebab with the version-aware drift `transformSet` applied. Any adoption path MUST reuse it, not hand-build the payload. | `routeros/mikrotik_serialize.go:117`, `:206-213` |
| User-visible logging | `ColorizedDebug` / `ColorizedMessage` write to the TF log (invisible without `TF_LOG`). Warnings returned in `diag.Diagnostics` appear in normal apply output. | `routeros/log.go:23,30` |

### Consequence

Two gaps must be closed before adoption can be written at all: **(a)** the resource layer cannot
currently tell *why* a create failed, and **(b)** there is no way to test any of this offline. Both
are prerequisites, not incidental refactors.

## 2. Device behaviour (RouterOS 7.21.5, verified on N-DC1-NBG1-SW1)

- Creating a bridge port for an interface that already has one fails with HTTP **400**, message
  `Bad Request`, detail **`device already added as bridge port`**.
- Deleting a bridge does **not** delete its port rows. 24 port rows survived, each still reserving its
  interface and each still causing the 400 above.
- An orphaned row reports its bridge as an internal id (observed: `bridge=*1C`), not a name — the
  referenced bridge no longer resolves by name *or* by id.
- Consequence for classification: "is the holding bridge alive?" MUST be answered by querying
  `/interface/bridge`, and MUST tolerate the held value being either a name or a `*`-prefixed id.

Not verified on other 7.x releases. Per Principle V the detail string and the id-vs-name behaviour are
recorded in [contracts/device-rest.md](./contracts/device-rest.md) with their verified version, and the
matcher is written so an unrecognized detail falls through to today's error rather than guessing.

## 3. Decisions

### D1 — Adoption is a create-*recovery* path, not a pre-create lookup

**Decision**: attempt `CreateItem` exactly as today. Only if it fails, *and* the failure is the
bridge-port conflict, *and* adoption is enabled, do we look the existing membership up and adopt it.

**Rationale**: with adoption off, the executed code path is identical to today's — same request, same
error, same text (Principle I, FR-011). It also costs zero extra device round-trips in the common
no-conflict case.

**Rejected**: *read-before-create* (always `ReadItemsFiltered` first, then create or adopt). Simpler to
reason about, but adds a device request to every single bridge-port create whether or not adoption is
enabled, and changes behaviour for users who did not opt in.

### D2 — Adoption is wired into the bridge port resource only, via a dedicated create function

**Decision**: add `AdoptOnConflictCreate(s)` in a new file `routeros/resource_actions_adopt.go` and use
it as the bridge port resource's `CreateContext`. It delegates to the unmodified `ResourceCreate` for
everything except the conflict branch. `ResourceCreate` itself is not touched.

**Rationale**: the spec scopes this to bridge port membership (Assumptions → Feature scope). A
dedicated wrapper is one new file plus a one-line change at the resource, which is the cherry-pickable
shape Principle II asks for, and it leaves a named seam another resource can adopt later without
inheriting the behaviour by accident.

**Rejected**: putting the branch inside `ResourceCreate`. One conditional would silently apply to every
resource in the provider — maximal blast radius for a single-resource problem.

### D3 — The transport gains a typed error; its `Error()` text is unchanged

**Decision**: introduce `type DeviceError struct { Method, URL, StatusCode int, Message, Detail string }`
with `Error()` producing **byte-identical** output to the current `fmt.Errorf` at
`mikrotik_client_rest.go:101`, plus a helper `AsDeviceError(error) (*DeviceError, bool)`. The REST
transport returns it instead of the bare `fmt.Errorf`.

**Rationale**: string-scraping a formatted message in the resource layer is exactly the class of
mistake Principle V calls a defect. A typed error is small, upstreamable on its own, and makes the
conflict test a field comparison (`StatusCode == 400 && Detail == …`). Keeping `Error()` identical
means no user-visible change and no test in the existing suite has to move (Principle I).

**Scope note**: the API transport (`mikrotik_client_api.go:50`) is **not** converted in this feature.
Adoption over the API transport therefore falls through to today's failure. This is a stated limitation
in the quickstart, not a silent gap — the deployment path uses REST.

**Rejected**: matching on the message string in the resource layer (fragile, duplicated per transport);
sentinel `errors.Is` values (needs the detail *text* anyway, so it buys nothing).

### D4 — One provider-level tri-state setting, not two booleans

**Decision**: `adopt_bridge_ports` on the provider, `TypeString`, `Optional`, values:

| Value | Adopts orphaned membership | Adopts membership already on the declared bridge | Re-points from a different **live** bridge |
| --- | --- | --- | --- |
| `off` *(default)* | no | no | no |
| `orphaned` | **yes** | **yes** | no |
| `any` | **yes** | **yes** | **yes** |

`DefaultFunc: MultiEnvDefaultFunc([]string{"ROS_ADOPT_BRIDGE_PORTS"}, "off")`,
`ValidateFunc: validation.StringInSlice([...], false)`.

**Rationale**: matches the fork's own precedent for provider-level switches
(`routeros/provider.go:104-129`) including the env default, so the deployment pipeline can enable it
without editing any Terraform module. A tri-state makes the FR-006 safety boundary a single ordered
axis and makes the nonsensical combination ("moving allowed but adoption disabled") unrepresentable.

**Rejected**: two provider booleans (`adopt_existing` + `allow_bridge_move`) — 4 states for 3 meanings;
resource-level attributes (would appear in every plan and state, and would require changing the
consuming `tfmodule-routeros-*` modules, which is the cost this design exists to avoid);
environment-variable-only (no Terraform-visible contract, untestable).

### D5 — The resource ID is set only after convergence succeeds

**Decision**: on the adoption path, `d.SetId()` is called **after** the `UpdateItem` that converges the
adopted row to the declaration. A convergence failure returns an error with **no** ID set.

**Rationale**: FR-009 forbids reporting success for an object that does not match its declaration. Two
ways to fail exist; they differ in what the *next* run does. Setting the ID first leaves a tainted
resource, and Terraform's remedy for a tainted resource is destroy-then-create — i.e. it would delete a
port row this run never successfully took ownership of (Principle IV). Leaving state empty means the
next run simply re-attempts adoption from a clean slate.

**Cost**: the row on the device may have been partially converged by the failed `UpdateItem`. Accepted:
it is unmanaged and re-adoptable, and RouterOS applies a PATCH atomically per row.

### D6 — Each adoption is reported as a `diag.Warning`

**Decision**: every adoption appends one warning diagnostic naming the interface, the bridge, the
classification (`orphaned` / `same-bridge` / `moved`), and the adopted row id.

**Rationale**: FR-010 and SC-006 require one visible entry per adoption. `ColorizedDebug` is invisible
without `TF_LOG=DEBUG`, so it cannot satisfy "recorded in the run output". Warnings show in ordinary
`terraform apply` output and do not fail the run.

**Rejected**: info-level logging (invisible in CI output); a summary count (SC-006 wants
per-membership, and "no silent adoptions" means each one is individually visible).

### D7 — Classification reads the device twice, and refuses ambiguity

**Decision**: on conflict, `ReadItemsFiltered(["interface=<declared>"], "/interface/bridge/port")`.

- 0 rows → **not** our conflict; return the original device error unchanged.
- \>1 rows → fail with a diagnostic; the device invariant "one membership per interface" is broken and
  guessing which row to adopt is unsafe.
- 1 row → resolve its `bridge` value against `/interface/bridge` (by `name`, then by `.id` if the value
  is `*`-prefixed). Not found → `orphaned`. Equal to the declared bridge → `same-bridge`. Found and
  different → `moved`, allowed only at `any`.

**Rationale**: fail-closed on every shape that isn't exactly one unambiguous row (Principle IV). The
0-row case returning the *original* error matters: it means a 400 with a similar detail that we
misclassified degrades to current behaviour rather than to a confusing new message.

### D8 — Offline tests use a new stub `Client`

**Decision**: add a table-driven stub implementing the 3-method `Client` interface in
`routeros/resource_actions_adopt_test.go` — it records requests and replays scripted responses,
including a 400 `DeviceError`. No `httptest` server, no live device.

**Rationale**: the interface is 3 methods (`routeros/mikrotik_client.go:20-24`), so a stub is cheaper
and more precise than an HTTP fake, and it lets a test assert *which* requests were issued — needed for
"no extra request when adoption is off" (FR-011) and "no delete was issued" (FR-004). The repo has no
such stub today; this is the first one, and it is confined to the new test file.

**Failing-first evidence plan** (Principle III, non-negotiable): the conflict-adoption test is written
against the current `DefaultCreate` wiring first and must fail with the 400, then pass once
`AdoptOnConflictCreate` is wired. Recorded in tasks as an explicit ordered pair.

**Class guard, not just the instance**: one test asserts that with `adopt_bridge_ports = "off"` the stub
sees *exactly* the requests today's code issues — the guard against any future change leaking
behaviour into the default path.

## 4. Resolved unknowns

| Unknown | Resolution |
| --- | --- |
| Can the conflict be detected without string matching? | Yes, via D3's typed error — status + detail as fields. |
| Can adoption avoid delete/recreate? | Yes. `UpdateItem` PATCHes the existing row by id (`mikrotik_crud.go:95-117`), satisfying FR-004. |
| Does adoption need new serialization? | No. `TerraformResourceDataToMikrotik` output is reused verbatim, so the drift `transformSet` still applies. |
| How does the pipeline enable this without module edits? | `ROS_ADOPT_BRIDGE_PORTS=orphaned` in the deploy environment (D4). |
| Does the bulk-read cache hide a just-adopted row? | No. `UpdateItem` invalidates the cache for that path/id (`mikrotik_crud.go:113`), and classification uses uncached `ReadItemsFiltered`. |
| Is the API transport covered? | No — explicitly out of scope for this feature (D3 scope note). |

## 5. Pre-existing condition: the offline suite aborts before it finishes

Measured on branch base `b62c425` with **no changes applied**, so this is inherited, not caused by this
feature. `go build ./...` is clean. `go test ./routeros/...` gives:

| | |
| --- | --- |
| Top-level tests declared before the abort | 20 |
| PASS | 18 |
| FAIL | 1 — `TestClientTransport_SendRequest`, needs `ROS_HOSTURL` / `ROS_USERNAME` (`routeros/provider_test.go:140`) |
| Abort | `Test_mikrotikResourceDataToTerraform` never reports a result |

The abort is a `log.Fatal` in library code: `driftObjects.GetDriftMap` calls `parseRouterOSVersion` and
`log.Fatal`s when it fails (`routeros/mikrotik_resource_drift_implementation.go:68-71`). With the
package-level `RouterOSVersion` unset, that test hits it and `log.Fatal` calls `os.Exit(1)`, killing the
whole test binary:

```text
2026/07/30 RouterOS version parts parsing error, strconv.ParseUint: parsing "": invalid syntax
FAIL    github.com/terraform-routeros/terraform-provider-routeros/routeros    0.020s
```

**Why this matters to this feature**, not just as trivia: Go compiles a package's test files in
alphabetical order, so everything declared after `mikrotik_serialize_test.go` — including
`resource_actions_adopt_test.go` — **never runs** in a bare `go test ./routeros/...`. Verification must
use a `-run` filter that excludes the aborting test, which is what
[quickstart.md](./quickstart.md) already prescribes.

Consequences recorded in tasks:

- **T001** captures the numbers above as the baseline.
- **T037**'s gate is *no new failures against that baseline*, not "the suite is green" — it was not
  green before this feature and making it green is out of scope.
- Fixing the `log.Fatal` is a genuine defect under the constitution's Device-Safety and
  no-untested-mapping principles, but it is a **separate change**: it sits in shared drift code, affects
  every resource, and folding it in here would violate Principle II. Raised, deliberately not taken.

## 6. Prototype and revert

The decisions above were validated by building a throwaway prototype of the foundational layer
(typed error, provider attribute, `ExtraParams` plumbing) and then **reverting it in full** — the
working tree is back at `b62c425` for all source files. Nothing in `routeros/` is modified by this
feature yet; implementation begins only after this spec is approved.

What the prototype confirmed, and that is worth keeping:

- The transport change is confined to one place. `routeros/mikrotik_client_rest.go:98-103` is the only
  site that builds the device-error message, so the typed error is a ~6-line edit with no other caller
  to update.
- `ExtraParams` has two construction sites (above); a single-site edit would leave the API transport
  with a zero-valued policy.
- `go build ./...` stays clean with the attribute, the `ExtraParams` field and the typed error in place,
  so no hidden compile-time coupling stands in the way.

## 7. Remaining risk

- **Detail-string drift across RouterOS versions.** If a future release rewords
  `device already added as bridge port`, adoption silently stops working and users see today's 400
  again. Mitigation: the string lives in one place with its verified version, and the fall-through is
  the unchanged upstream error, never a wrong action. A device-behaviour note is required in the
  contract file.
- **`any` is genuinely traffic-affecting.** Mitigated by it being neither the default nor reachable
  from the base opt-in, and by SC-008's explicit negative test.
