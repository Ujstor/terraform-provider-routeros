# Quickstart: Adopt an existing bridge port membership

**Feature**: `001-bridge-port-adopt` | **Date**: 2026-07-30

Written against the design in [plan.md](./plan.md). Until implementation lands this describes intended
behaviour, not shipped behaviour.

## Enable it

Two equivalent ways. The environment variable is the one to use from a deployment pipeline, because it
needs no change to any Terraform module.

```hcl
provider "routeros" {
  hosturl  = "https://10.48.80.1:8244"
  username = var.ros_user
  password = var.ros_password
  insecure = true

  adopt_bridge_ports = "orphaned"
}
```

```bash
export ROS_ADOPT_BRIDGE_PORTS=orphaned
terraform apply
```

| Value | Use it when |
| --- | --- |
| `off` *(default)* | Shared or production devices. Conflicts fail with an actionable diagnostic. |
| `orphaned` | Onboarding a device, or re-applying after state loss. Adopts orphaned rows and rows already on the declared bridge. **Start here.** |
| `any` | You explicitly intend to re-point a port away from a bridge that is currently in service. Traffic-affecting. |

## What a run looks like

Onboarding a switch whose 23 interfaces are all held by orphaned port rows, with
`adopt_bridge_ports = "orphaned"`:

```text
Warning: Adopted existing bridge port ether5 into br0

  Bridge port *1C for ether5 existed on the device (orphaned bridge) and was converged to this
  configuration instead of being created. Previous bridge: *1C.

… one warning per adopted membership …

Apply complete! Resources: 23 added, 0 changed, 0 destroyed.
```

Run it again immediately: `No changes. Your infrastructure matches the configuration.` (SC-002).

Same device with the default `off`:

```text
Error: Interface ether5 is held by a bridge port whose bridge no longer exists

  The device holds ether5 as a port of *1C, which is not an existing bridge — the port row outlived
  its bridge and still reserves the interface. Set adopt_bridge_ports = "orphaned" on the provider
  (env ROS_ADOPT_BRIDGE_PORTS) to adopt it into br0, or remove the port row on the device.
```

Full diagnostic set: [contracts/provider-schema.md](./contracts/provider-schema.md) §2.

## Reproduce the conflict on a lab device

RouterOS 7.21.5. **Confirm management does not arrive over a bridge member first** — on the reference
device it is routed over `ether24`, which is what makes this safe remotely.

```routeros
/interface/bridge/print detail
/interface/bridge/port/print detail where interface=ether5
```

To create the orphan case deliberately:

```routeros
/interface/bridge/add name=scratch
/interface/bridge/port/add bridge=scratch interface=ether5
/interface/bridge/remove [find name=scratch]
/interface/bridge/port/print detail where interface=ether5
# the row survives, with bridge=*NN referring to the removed bridge
```

Then `terraform apply` a configuration declaring `ether5` on a different bridge.

## Run the tests

```bash
go build ./...
go test ./routeros/... -run 'Adopt|DeviceError' -v
```

No device or credentials are required — the adoption tests drive a stub `Client`
(`routeros/resource_actions_adopt_test.go`). The `resource_*_test.go` acceptance tests still need a live
device and are unaffected.

**The `-run` filter is required, not a convenience.** A bare `go test ./routeros/...` does not reach
these tests: `Test_mikrotikResourceDataToTerraform` triggers a `log.Fatal` in
`routeros/mikrotik_resource_drift_implementation.go:68-71` when the package-level `RouterOSVersion` is
unset, which calls `os.Exit(1)` and kills the test binary. Everything compiled after
`mikrotik_serialize_test.go` — alphabetically, that includes `resource_actions_adopt_test.go` — is
simply never executed. This is pre-existing at `b62c425` and out of scope here; see
[research.md](./research.md) §5.

## Verify the failing-first evidence

Principle III requires the order to be demonstrated, not asserted. Before the wiring change lands:

```bash
go test ./routeros/... -run TestAdoptOnConflict_OrphanedRowIsAdopted   # MUST fail: 400 conflict
```

After it lands, the same command passes and
`go test ./routeros/... -run TestAdoptDisabled_RequestSequenceUnchanged` proves the default path did not
move.

## Limitations

- **REST transport only.** The API transport does not produce typed errors yet, so adoption does not
  engage there; behaviour is unchanged.
- **Bridge port membership only.** Other object classes that reject creation because an equivalent
  object exists are out of scope.
- **Does not clean up factory defaults.** Default addresses, DHCP client and firewall rules are a
  pipeline-preflight concern, not this feature's.
- **`any` is destructive by design.** It re-points live ports. It is not reachable from `orphaned`.
