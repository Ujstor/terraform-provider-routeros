package routeros

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

/*
{
    ".about": "mode: AP, SSID: wlan, channel: 2462/n",
    ".id": "*1",
    "arp": "enabled",
    "arp-timeout": "auto",
    "bound": "true",
    "configuration": "cfg1",
    "configuration.manager": "capsman",
    "configuration.mode": "ap",
    "default-name": "wifi1",
    "disabled": "false",
    "inactive": "false",
    "l2mtu": "1560",
    "mac-address": "00:00:00:00:00:00",
    "master": "true",
    "name": "wifi1",
    "radio-mac": "00:00:00:00:00:00",
    "running": "true",
    "security.connect-priority": "0"
}
*/

// https://help.mikrotik.com/docs/display/ROS/WiFi#WiFi-Miscellaneousproperties
// https://help.mikrotik.com/docs/display/ROS/WiFi#WiFi-Read-onlyproperties
func ResourceWifi() *schema.Resource {
	resSchema := map[string]*schema.Schema{
		MetaResourcePath: PropResourcePath("/interface/wifi"),
		MetaId:           PropId(Id),
		MetaTransformSet: PropTransformSet("aaa.config: aaa", "channel.config: channel", "configuration.config: configuration",
			"datapath.config: datapath", "interworking.config: interworking", "security.config: security", "steering.config: steering"),

		"aaa": {
			Type:             schema.TypeMap,
			Optional:         true,
			Elem:             &schema.Schema{Type: schema.TypeString},
			Description:      "AAA inline settings.",
			ValidateDiagFunc: ValidationMapKeyNames,
			DiffSuppressFunc: WifiInlineMapDiffSuppress,
		},
		KeyArp:        PropArpRw,
		KeyArpTimeout: PropArpTimeoutRw,
		"bound": {
			Type:        schema.TypeBool,
			Computed:    true,
			Description: "A flag whether the interface is currently available for the WiFi manager.",
		},
		"channel": {
			Type:             schema.TypeMap,
			Optional:         true,
			Elem:             &schema.Schema{Type: schema.TypeString},
			Description:      "Channel inline settings.",
			ValidateDiagFunc: ValidationMapKeyNames,
			DiffSuppressFunc: WifiInlineMapDiffSuppress,
		},
		"configuration": {
			Type:             schema.TypeMap,
			Optional:         true,
			Elem:             &schema.Schema{Type: schema.TypeString},
			Description:      "Configuration inline settings.",
			ValidateDiagFunc: ValidationMapKeyNames,
			DiffSuppressFunc: WifiInlineMapDiffSuppress,
		},
		"datapath": {
			Type:             schema.TypeMap,
			Optional:         true,
			Elem:             &schema.Schema{Type: schema.TypeString},
			Description:      "Datapath inline settings.",
			ValidateDiagFunc: ValidationMapKeyNames,
			DiffSuppressFunc: WifiInlineMapDiffSuppress,
		},
		KeyComment:     PropCommentRw,
		KeyDefaultName: PropDefaultNameRo("The interface's default name."),
		"deprioritize_unii_3_4": {
			Type:     schema.TypeBool,
			Optional: true,
			Description: "Whether to assign lower priority to channels with a control frequency of 5720 or 5825-5885 " +
				"MHz. These channels are unsupported by some client devices, making their automatic selection " +
				"undesirable. Defaults to `yes` in ETSI regulatory domains, elsewhere to `no`.",
			DiffSuppressFunc: AlwaysPresentNotUserProvided,
		},
		KeyDisabled: PropDisabledRw,
		"disable_running_check": {
			Type:             schema.TypeBool,
			Optional:         true,
			Description:      "An option to set the running property to true if it is not disabled.",
			DiffSuppressFunc: AlwaysPresentNotUserProvided,
		},
		"inactive": {
			Type:        schema.TypeBool,
			Computed:    true,
			Description: "A flag whether the interface is currently inactive.",
		},
		"interworking": {
			Type:             schema.TypeMap,
			Optional:         true,
			Elem:             &schema.Schema{Type: schema.TypeString},
			Description:      "Interworking inline settings.",
			ValidateDiagFunc: ValidationMapKeyNames,
			DiffSuppressFunc: WifiInlineMapDiffSuppress,
		},
		KeyL2Mtu: PropL2MtuRw,
		"mac_address": {
			Type:             schema.TypeString,
			Description:      "MAC address (BSSID) to use for the interface.",
			Optional:         true,
			DiffSuppressFunc: AlwaysPresentNotUserProvided,
		},
		"master": {
			Type:        schema.TypeBool,
			Computed:    true,
			Description: "A flag whether the interface is not a virtual one.",
		},
		"master_interface": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The corresponding master interface of the virtual one.",
		},
		"mtu": {
			Type:             schema.TypeInt,
			Optional:         true,
			Description:      "Layer3 maximum transmission unit",
			ValidateFunc:     validation.IntBetween(32, 2290),
			DiffSuppressFunc: AlwaysPresentNotUserProvided,
		},
		KeyName: PropName("Name of the interface."),
		"radio_mac": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The MAC address of the associated radio.",
		},
		"running": {
			Type:        schema.TypeBool,
			Computed:    true,
			Description: "A flag whether the interface has established a link to another device.",
		},
		"security": {
			Type:             schema.TypeMap,
			Optional:         true,
			Elem:             &schema.Schema{Type: schema.TypeString},
			Description:      "Security inline settings.",
			ValidateDiagFunc: ValidationMapKeyNames,
			DiffSuppressFunc: WifiInlineMapDiffSuppress,
		},
		"steering": {
			Type:             schema.TypeMap,
			Optional:         true,
			Elem:             &schema.Schema{Type: schema.TypeString},
			Description:      "Steering inline settings.",
			ValidateDiagFunc: ValidationMapKeyNames,
			DiffSuppressFunc: WifiInlineMapDiffSuppress,
		},
	}

	return &schema.Resource{
		Description: `*<span style="color:red">This resource requires a minimum version of RouterOS 7.13.</span>*

Master (physical) interfaces already exist on the device and cannot be created or deleted via the API.
Terraform adopts them on create (update in place) and removes them from state only on destroy.
Virtual (slave) interfaces with ` + "`master_interface`" + ` are created and deleted normally.`,
		// Interaction with mixed types of elements within a single resource.
		// In this case, there are physical and virtual interfaces that need to be created and deleted in different ways.
		CreateContext: func(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
			metadata := GetMetadata(resSchema)
			filter := buildReadFilter(map[string]interface{}{"name": d.Get("name")})
			items, err := ReadItemsFiltered(filter, metadata.Path, m.(Client))
			if err != nil {
				return diag.FromErr(err)
			}

			var diags diag.Diagnostics
			if items == nil || len(*items) == 0 {
				// No interface with the specified name was found. Adding...
				diags = ResourceCreate(ctx, resSchema, d, m)
			} else {
				// An interface with the specified name is found. There are two options:
				//		it is a master and then we will update it with existing settings,
				//		or it is virtual and needs to be imported.
				iface := (*items)[0]
				if master, ok := iface["master"]; ok {
					if strings.ToLower(master) == "true" {
						// It's a master (physical) interface.
						d.SetId(iface.GetID(Id))
						diags = ResourceUpdate(ctx, resSchema, d, m)
					} else {
						diags = diag.Diagnostics{diag.Diagnostic{
							Severity: diag.Error,
							Summary:  fmt.Sprintf("A virtual interface named '%v' already exists", d.Get("name")),
						}}
					}
				} else {
					diags = diag.Diagnostics{diag.Diagnostic{
						Severity: diag.Error,
						Summary: fmt.Sprintf("The Mikrotik resource (%v print where name=%v) does not contain "+
							"'master' attribute in the response",
							metadata.Path, d.Get("name")),
					}}
				}
			}

			if diags.HasError() {
				return diags
			}

			return ResourceRead(ctx, resSchema, d, m)
		},

		ReadContext:   DefaultRead(resSchema),
		UpdateContext: DefaultUpdate(resSchema),
		DeleteContext: func(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
			if d.Get("master").(bool) {
				// It's a master (physical) interface.
				return SystemResourceDelete(ctx, resSchema, d, m)
			}

			return ResourceDelete(ctx, resSchema, d, m)
		},

		Importer: &schema.ResourceImporter{
			StateContext: ImportStateCustomContext(resSchema),
		},

		Schema: resSchema,
	}
}
