package routeros

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// RouterOS keeps egress-mirror0/1 and ingress-mirror0/1 as an ORDERED pair
// "<port-or-trunk>,<format>" (e.g. "switch1-cpu,modified"). The provider must
// send them back in that order, otherwise the router parses the format word
// as the port and answers 400 "invalid value for argument port-or-trunk".
// Declared as schema.TypeSet the pair was re-joined in hash order
// ("modified,switch1-cpu"); it has to stay an ordered list.
func TestSwitchCrsMirrorPairsKeepDeviceOrder(t *testing.T) {
	s := ResourceInterfaceEthernetSwitchCrs().Schema

	// The DiffSuppressFuncs need a real raw config, which the test harness
	// does not provide - build the ResourceData from a copy without them.
	plain := make(map[string]*schema.Schema, len(s))
	for k, v := range s {
		c := *v
		c.DiffSuppressFunc = nil
		plain[k] = &c
	}
	d := schema.TestResourceDataRaw(t, plain, map[string]interface{}{
		"name":            "switch1",
		"egress_mirror0":  []interface{}{"switch1-cpu", "modified"},
		"egress_mirror1":  []interface{}{"switch1-cpu", "modified"},
		"ingress_mirror0": []interface{}{"switch1-cpu", "unmodified"},
		"ingress_mirror1": []interface{}{"switch1-cpu", "unmodified"},
	})

	for field, want := range map[string]string{
		"egress_mirror0":  "switch1-cpu,modified",
		"egress_mirror1":  "switch1-cpu,modified",
		"ingress_mirror0": "switch1-cpu,unmodified",
		"ingress_mirror1": "switch1-cpu,unmodified",
	} {
		if s[field].Type != schema.TypeList {
			t.Errorf("%s: must be schema.TypeList (ordered), got %v", field, s[field].Type)
		}

		// Exactly what TerraformResourceDataToMikrotik does with each type.
		var got string
		switch v := d.Get(field).(type) {
		case *schema.Set:
			got = ListToString(v.List())
		default:
			got = ListToString(v)
		}
		if got != want {
			t.Errorf("%s: serialised as %q, want %q (port-or-trunk must come first)", field, got, want)
		}
	}
}
