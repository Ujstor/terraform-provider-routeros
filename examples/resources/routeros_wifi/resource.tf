# If you need to add a reference to an existing configuration, each inline section contains a `config` parameter 
# where you can specify the name of the actual resource.
# configuration = {
#   config = routeros_wifi_configuration.my-config.name
# }

# Master (physical) interface — adopted and updated in place; destroy only removes from state.
resource "routeros_wifi" "wifi1" {
  configuration = {
    manager = "capsman"
  }
  name = "wifi1"
}

# Virtual (slave) interface — created and deleted via the API.
resource "routeros_wifi" "wifi1_guest" {
  master_interface = routeros_wifi.wifi1.name
  name             = "wifi1-guest"
  configuration = {
    ssid = "guest"
  }
}
