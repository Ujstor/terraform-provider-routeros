# routeros_wifi_capsman (Resource)
*<span style="color:red">This resource requires a minimum version of RouterOS 7.13.</span>*

## Example Usage
```terraform
resource "routeros_wifi_capsman" "settings" {
  enabled        = true
  interfaces     = ["bridge1"]
  upgrade_policy = "suggest-same-version"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `ca_certificate` (String) Device CA certificate.
- `certificate` (String) Device certificate.
- `enabled` (Boolean) Disable or enable CAPsMAN functionality.
- `interfaces` (List of String) List of interfaces on which CAPsMAN will listen for CAP connections.
- `package_path` (String) Folder location for the RouterOS packages. For example, use '/upgrade' to specify the upgrade folder from the files section. If empty string is set, CAPsMAN can use built-in RouterOS packages, note that in this case only CAPs with the same architecture as CAPsMAN will be upgraded.
- `require_peer_certificate` (Boolean) Require all connecting CAPs to have a valid certificate.
- `upgrade_policy` (String) Upgrade policy options.

### Read-Only

- `generated_ca_certificate` (String) Generated CA certificate.
- `generated_certificate` (String) Generated CAPsMAN certificate.
- `id` (String) The ID of this resource.

## Import
Import is supported using the following syntax:
```shell
terraform import routeros_wifi_capsman.settings .
```