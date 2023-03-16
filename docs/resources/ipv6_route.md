# routeros_ipv6_route (Resource)


## Example Usage
```terraform
resource "routeros_ipv6_route" "a_route" {
  dst_address = "::/0"
  gateway     = "2001:DB8:1000::1"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `dst_address` (String) IP prefix of route, specifies destination addresses that this route can be used for.
- `gateway` (String) Array of IP addresses or interface names. Specifies which host or interface packets should be sent to (IP | interface | IP%interface | IP@table[, IP | string, [..]]).

### Optional

- `blackhole` (Boolean) It's a blackhole route.
- `comment` (String)
- `disabled` (Boolean)
- `distance` (Number) Value used in route selection. Routes with smaller distance value are given preference.
- `pref_src` (String) Which of the local IP addresses to use for locally originated packets that are sent via this route. Value of this property has no effect on forwarded packets. If value of this property is set to IP address that is not local address of this router then the route will be inactive (in ROS v6, ROS v7 allows IP spoofing).
- `routing_table` (String) Routing table this route belongs to.
- `scope` (Number) Used in nexthop resolution. Route can resolve nexthop only through routes that have scope less than or equal to the target-scope of this route.
- `target_scope` (Number) Used in nexthop resolution. This is the maximum value of scope for a route through which a nexthop of this route can be resolved.
- `vrf_interface` (String) VRF interface name.

### Read-Only

- `active` (Boolean) A flag indicates whether the route is elected as Active and eligible to be added to the FIB.
- `dynamic` (Boolean) Configuration item created by software, not by management interface. It is not exported, and cannot be directly modified.
- `ecmp` (Boolean) A flag indicates whether the route is added as an Equal-Cost Multi-Path route in the FIB.
- `hw_offloaded` (Boolean) Indicates whether the route is eligible to be hardware offloaded on supported hardware.
- `id` (String) The ID of this resource.
- `immediate_gw` (String) Shows actual (resolved) gateway and interface that will be used for packet forwarding.
- `inactive` (Boolean)
- `static` (Boolean)
- `suppress_hw_offload` (Boolean)

## Import
Import is supported using the following syntax:
```shell
#The ID can be found via API or the terminal
#The command for the terminal is -> :put [/ipv6/route get [print show-ids]]
terraform import routeros_ipv6_route.a_route "*0"
```