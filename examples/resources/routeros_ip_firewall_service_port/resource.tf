# Disable the SIP ALG (NAT helper); the entry is built into RouterOS and is
# adopted by name, never created.
resource "routeros_ip_firewall_service_port" "sip" {
  name     = "sip"
  disabled = true
}
