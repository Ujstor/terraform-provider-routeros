package routeros

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var (
	testResource = schema.Resource{
		Schema: map[string]*schema.Schema{
			MetaResourcePath: PropResourcePath("/test/resource"),
			MetaId:           PropId(Id),
			"string": {
				Type: schema.TypeString,
			},
			"float": {
				Type: schema.TypeFloat,
			},
			"int": {
				Type: schema.TypeInt,
			},
			"bool": {
				Type: schema.TypeBool,
			},
			"computed": {
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}

	testDatasource = schema.Resource{
		Schema: map[string]*schema.Schema{
			MetaResourcePath: PropResourcePath("/test/resource"),
			MetaId:           PropId(Id),
			"test_name": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"string": {
							Type: schema.TypeString,
						},
						"float": {
							Type: schema.TypeFloat,
						},
						"int": {
							Type: schema.TypeInt,
						},
						"bool": {
							Type: schema.TypeBool,
						},
					},
				},
			},
		},
	}
)

func Test_mikrotikResourceDataToTerraform(t *testing.T) {
	originalVersion := RouterOSVersion
	RouterOSVersion = "7.16"
	t.Cleanup(func() { RouterOSVersion = originalVersion })

	testItem := MikrotikItem{".id": "*39", "string": "string12345", "float": "0.01", "int": "10", "bool": "true"}

	testResourceData := testResource.TestResourceData()
	expectedRes := map[string]interface{}{"string": "string12345", "float": 0.01, "int": 10, "bool": true}

	err := MikrotikResourceDataToTerraform(testItem, testResource.Schema, testResourceData)
	if err != nil {
		t.Errorf("decoding err: %v", err)
	}

	for key, expected := range expectedRes {
		actual := testResourceData.Get(key)

		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("bad: expected:%#v\nactual:%#v", expected, actual)
		}
	}

}

func Test_mikrotikResourceDataToTerraform_clearsAbsentTypeMaps(t *testing.T) {
	// Mirrors routeros_wifi inline maps: after a device reset ROS omits
	// configuration.*/channel.* entirely; Read must clear prior state.
	originalVersion := RouterOSVersion
	RouterOSVersion = "7.16"
	t.Cleanup(func() { RouterOSVersion = originalVersion })

	res := schema.Resource{
		Schema: map[string]*schema.Schema{
			MetaResourcePath: PropResourcePath("/interface/wifi"),
			MetaId:           PropId(Id),
			MetaTransformSet: PropTransformSet(
				"channel.config: channel",
				"configuration.config: configuration",
			),
			"name": {
				Type: schema.TypeString,
			},
			"configuration": {
				Type: schema.TypeMap,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			"channel": {
				Type: schema.TypeMap,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			"security": {
				Type: schema.TypeMap,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
		},
	}

	rd := res.TestResourceData()
	rd.SetId("*2")
	if err := rd.Set("name", "wifi1"); err != nil {
		t.Fatal(err)
	}
	if err := rd.Set("configuration", map[string]interface{}{
		"config": "thegeraet-network__radio-5g",
		"ssid":   "TheGeraet",
		"mode":   "ap",
	}); err != nil {
		t.Fatal(err)
	}
	if err := rd.Set("channel", map[string]interface{}{
		"config": "radio-5g",
		"band":   "5ghz-ax",
	}); err != nil {
		t.Fatal(err)
	}
	if err := rd.Set("security", map[string]interface{}{
		"config":               "thegeraet-security",
		"authentication_types": "wpa2-psk,wpa3-psk",
	}); err != nil {
		t.Fatal(err)
	}

	// Reset-like API payload: iface exists, dotted map fields absent.
	item := MikrotikItem{".id": "*2", "name": "wifi1"}
	if diags := MikrotikResourceDataToTerraform(item, res.Schema, rd); diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}

	for _, key := range []string{"configuration", "channel", "security"} {
		actual := rd.Get(key)
		m, ok := actual.(map[string]interface{})
		if !ok {
			t.Fatalf("%s: expected map, got %#v", key, actual)
		}
		if len(m) != 0 {
			t.Fatalf("%s: expected cleared map after absent API fields, got %#v", key, m)
		}
	}

	// When API returns map fields again, they must be restored (and transform applied).
	item = MikrotikItem{
		".id":                 "*2",
		"name":                "wifi1",
		"configuration.ssid":  "TheGeraet",
		"configuration.mode":  "ap",
		"channel.band":        "5ghz-ax",
		"channel":             "radio-5g",
	}
	if diags := MikrotikResourceDataToTerraform(item, res.Schema, rd); diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}

	cfg := rd.Get("configuration").(map[string]interface{})
	if cfg["ssid"] != "TheGeraet" || cfg["mode"] != "ap" {
		t.Fatalf("configuration not restored: %#v", cfg)
	}
	ch := rd.Get("channel").(map[string]interface{})
	if ch["band"] != "5ghz-ax" || ch["config"] != "radio-5g" {
		t.Fatalf("channel not restored: %#v", ch)
	}
	sec := rd.Get("security").(map[string]interface{})
	if len(sec) != 0 {
		t.Fatalf("security should stay cleared when still absent: %#v", sec)
	}
}

func Test_terraformResourceDataToMikrotik(t *testing.T) {

	expected := MikrotikItem{"string": "string12345", "float": "0.01", "int": "10", "bool": "yes"}

	testResourceData := testResource.TestResourceData()
	testResourceData.SetId("*39")
	testResourceData.Set("string", "string12345")
	testResourceData.Set("float", 0.01)
	testResourceData.Set("int", 10)
	testResourceData.Set("bool", true)

	actual, _ := TerraformResourceDataToMikrotik(testResource.Schema, testResourceData)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: expected:%#v\nactual:%#v", expected, actual)
	}
}

func Test_mikrotikResourceDataToTerraformDatasource(t *testing.T) {
	originalVersion := RouterOSVersion
	RouterOSVersion = "7.16"
	t.Cleanup(func() { RouterOSVersion = originalVersion })

	testItems := []MikrotikItem{
		{"string": "string12345", "float": "0.01", "int": "10", "bool": "yes"},
		{"string": "12345string", "float": "0.02", "int": "20", "bool": "no"},
	}

	testResourceData := testDatasource.TestResourceData()
	expectedRes := []map[string]interface{}{
		{MetaResourcePath: "", MetaId: 0, "string": "string12345", "float": 0.01, "int": 10, "bool": true},
		{MetaResourcePath: "", MetaId: 0, "string": "12345string", "float": 0.02, "int": 20, "bool": false},
	}

	err := MikrotikResourceDataToTerraformDatasource(&testItems, "test_name", testDatasource.Schema, testResourceData)
	if err != nil {
		t.Errorf("decoding err: %v", err)
	}

	for i, rec := range testResourceData.Get("test_name").([]interface{}) {
		for key, actual := range rec.(map[string]interface{}) {
			if !reflect.DeepEqual(actual, expectedRes[i][key]) {
				t.Fatalf("bad: (key: %v) expected:%#v\tactual:%#v", key, expectedRes[i][key], actual)
			}
		}
	}
}

func Test_loadTransformSet(t *testing.T) {
	testData := []struct {
		s       string
		reverse bool
	}{
		{toQuotedCommaSeparatedString("channel: channel.config", "datapath: datapath.config"), false},
		{toQuotedCommaSeparatedString("mikrotik-field-name : schema-field-name"), false},
		{toQuotedCommaSeparatedString("channel: channel.config", "datapath: datapath.config"), true},
		{toQuotedCommaSeparatedString("mikrotik-field-name:schema-field-name"), true},
	}

	expected := []map[string]string{
		{"channel": "channel.config", "datapath": "datapath.config"},
		{"mikrotik-field-name": "schema-field-name"},
		{"channel.config": "channel", "datapath.config": "datapath"},
		{"schema-field-name": "mikrotik-field-name"},
	}

	for i, actual := range testData {
		if !reflect.DeepEqual(loadTransformSet(actual.s, actual.reverse), expected[i]) {
			t.Fatalf("bad: (item: %v) expected:%#v\tactual:%#v", i, expected[i], loadTransformSet(actual.s, actual.reverse))
		}
	}
}

func Test_loadSkipFields(t *testing.T) {
	testData := []struct {
		s string
	}{
		{toQuotedCommaSeparatedString("name")},
		{toQuotedCommaSeparatedString("name", "rx_1024_1518", "rx_128_255", "rx_1519_max", "rx_256_511", "rx_512_1023", "rx_64")},
	}

	expected := []map[string]struct{}{
		{"name": struct{}{}},
		{"name": struct{}{}, "rx_1024_1518": struct{}{}, "rx_128_255": struct{}{}, "rx_1519_max": struct{}{},
			"rx_256_511": struct{}{}, "rx_512_1023": struct{}{}, "rx_64": struct{}{}},
	}

	for i, actual := range testData {
		if !reflect.DeepEqual(loadSkipFields(actual.s), expected[i]) {
			t.Fatalf("bad: (item: %v) expected:%#v\tactual:%#v", i, expected[i], loadSkipFields(actual.s))
		}
	}
}
