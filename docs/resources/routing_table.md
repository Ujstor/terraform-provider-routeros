# routeros_routing_table (Resource)


## Example Usage
```terraform
resource "routeros_routing_table" "test_table" {
    name    = "to_ISP1"
    fib     = false
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `comment` (String)
- `disabled` (Boolean)
- `fib` (Boolean) fib parameter should be specified if the routing table is intended to push routes to the FIB.
- `name` (String) Routing table name.

### Read-Only

- `dynamic` (Boolean) Configuration item created by software, not by management interface. It is not exported, and cannot be directly modified.
- `id` (String) The ID of this resource.
- `invalid` (Boolean)

## Import
Import is supported using the following syntax:
```shell
#The ID can be found via API or the terminal
#The command for the terminal is -> :put [/routing/table get [print show-ids]]
terraform import routeros_routing_table.test_table "*0"
```