package routeros

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const testIpFirewallServicePort = "routeros_ip_firewall_service_port.test"

func TestAccIpFirewallServicePortTest_basic(t *testing.T) {
	for _, name := range testNames {
		t.Run(name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck: func() {
					testAccPreCheck(t)
					testSetTransportEnv(t, name)
				},
				ProviderFactories: testAccProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: testAccIpFirewallServicePortConfig("true"),
						Check: resource.ComposeTestCheckFunc(
							testResourcePrimaryInstanceId(testIpFirewallServicePort),
							resource.TestCheckResourceAttr(testIpFirewallServicePort, "name", "sip"),
							resource.TestCheckResourceAttr(testIpFirewallServicePort, "disabled", "true"),
							resource.TestCheckResourceAttr(testIpFirewallServicePort, "ports", "5060,5061"),
						),
					},
					{
						Config: testAccIpFirewallServicePortConfig("false"),
						Check: resource.ComposeTestCheckFunc(
							testResourcePrimaryInstanceId(testIpFirewallServicePort),
							resource.TestCheckResourceAttr(testIpFirewallServicePort, "name", "sip"),
							resource.TestCheckResourceAttr(testIpFirewallServicePort, "disabled", "false"),
						),
					},
				},
			})
		})
	}
}

func testAccIpFirewallServicePortConfig(param string) string {
	return fmt.Sprintf(`%v

resource "routeros_ip_firewall_service_port" "test" {
  name     = "sip"
  disabled = %v
}
`, providerConfig, param)
}
