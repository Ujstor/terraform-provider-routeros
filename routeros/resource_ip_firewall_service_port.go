package routeros

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

/*
  /ip/firewall/service-port print  (RouterOS 7.23.3)
  {
    ".id": "*1",
    "disabled": "false",
    "invalid": "false",
    "name": "ftp",
    "ports": "21"
  },
  {
    ".id": "*5",
    "disabled": "false",
    "invalid": "false",
    "name": "sip",
    "ports": "5060,5061",
    "sip-direct-media": "true",
    "sip-timeout": "1h"
  },
  {
    ".id": "*8",
    "disabled": "false",
    "invalid": "false",
    "name": "udplite"
  }
*/

// https://help.mikrotik.com/docs/spaces/ROS/pages/3211299/NAT#NAT-ServicePorts
//
// The NAT helpers (connection-tracking ALGs) are built into RouterOS: the entries
// cannot be created or removed, only modified. Create/Update look the entry up by
// its name and set the given attributes; Delete forgets the entry from the state
// without touching the device (the same contract as routeros_ip_service and
// routeros_ip_hotspot_service_port).
func ResourceIpFirewallServicePort() *schema.Resource {
	resSchema := map[string]*schema.Schema{
		MetaResourcePath: PropResourcePath("/ip/firewall/service-port"),
		MetaId:           PropId(Id),
		MetaSkipFields:   PropSkipFields("name"),

		KeyDisabled: PropDisabledRw,
		KeyInvalid:  PropInvalidRo,
		"name": {
			Type:     schema.TypeString,
			Required: true,
			ForceNew: true,
			Description: "Name of the built-in NAT helper (`ftp`, `tftp`, `irc`, `h323`, `sip`, `pptp`, `rtsp`, " +
				"`udplite`, `dccp`, `sctp` on RouterOS 7.23). The entry must already exist on the device.",
		},
		"ports": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
			Description: "Comma-separated list of ports the helper inspects (`5060,5061`). Helpers without a port " +
				"(`h323`, `pptp`, `udplite`, `dccp`, `sctp`) leave it empty. Unset keeps the device value.",
		},
		"sip_direct_media": {
			Type:        schema.TypeBool,
			Optional:    true,
			Computed:    true,
			Description: "`sip` only: allow the media stream to bypass the router when both endpoints are directly reachable.",
		},
		"sip_timeout": {
			Type:             schema.TypeString,
			Optional:         true,
			Computed:         true,
			Description:      "`sip` only: connection-tracking timeout for SIP sessions (`1h`).",
			DiffSuppressFunc: TimeEqual,
		},
	}

	return &schema.Resource{
		CreateContext: DefaultCreateUpdate(resSchema),
		ReadContext:   DefaultRead(resSchema),
		UpdateContext: DefaultCreateUpdate(resSchema),
		DeleteContext: DefaultSystemDelete(resSchema),

		Importer: &schema.ResourceImporter{
			StateContext: ImportStateCustomContext(resSchema),
		},

		Schema: resSchema,
	}
}
