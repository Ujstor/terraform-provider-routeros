package routeros

import (
	"strings"
	"testing"
)

// The serializer resolves a drift rename with
//
//	mikrotikKebabName := SnakeToKebab(terraformSnakeName)
//	if new, ok := transformSet[terraformSnakeName]; ok { ... }
//
// so a drift entry's `tf` value must be the snake_case Terraform schema
// attribute name. An entry written in MikroTik kebab-case never matches, and
// the rename silently does nothing - the symptom is RouterOS rejecting the
// pre-rename property with "unknown parameter <name>".
func TestDriftAttributesUseSnakeCaseTFNames(t *testing.T) {
	for _, obj := range driftAttributeSlice {
		for path, attrs := range obj.Resources {
			for _, attr := range attrs {
				if strings.Contains(attr.TF, "-") {
					t.Errorf("drift %s %s: tf=%q must be the snake_case schema attribute, not kebab-case "+
						"(the transformSet lookup key is the schema field name, so this entry can never match)",
						obj.ros, path, attr.TF)
				}
			}
		}
	}
}

// Every drift `tf` name must exist as an attribute in the schema of the
// resource that owns the path, otherwise the entry is dead code.
func TestDriftAttributesExistInResourceSchema(t *testing.T) {
	// resource path -> set of schema attribute names, from the provider itself.
	byPath := map[string]map[string]bool{}
	for _, res := range Provider().ResourcesMap {
		p, ok := res.Schema[MetaResourcePath]
		if !ok || p.Default == nil {
			continue
		}
		path, _ := p.Default.(string)
		if path == "" {
			continue
		}
		if _, seen := byPath[path]; !seen {
			byPath[path] = map[string]bool{}
		}
		for attr := range res.Schema {
			byPath[path][attr] = true
		}
	}

	for _, obj := range driftAttributeSlice {
		for path, attrs := range obj.Resources {
			attrsInSchema, ok := byPath[path]
			if !ok {
				t.Logf("drift %s %s: no resource in ResourcesMap declares this path - skipping", obj.ros, path)
				continue
			}
			for _, attr := range attrs {
				if !attrsInSchema[attr.TF] {
					t.Errorf("drift %s %s: tf=%q is not an attribute of the resource schema for that path",
						obj.ros, path, attr.TF)
				}
			}
		}
	}
}

// Guards the RouterOS 7.21 /ip/ssh rename specifically: 7.21+ must serialize
// always_allow_password_login as password-authentication, and pre-7.21 must
// keep the original property.
func TestDriftIpSSHPasswordAuthentication(t *testing.T) {
	const schemaAttr = "always_allow_password_login"

	for _, tc := range []struct {
		ros  string
		want string
	}{
		{"7.20.1", "always-allow-password-login"},
		{"7.21", "password-authentication"},
		{"7.21.5", "password-authentication"},
		{"7.22", "password-authentication"},
	} {
		drift := driftAttributeSlice.GetDriftMap(tc.ros, "/ip/ssh", false)

		// Mirror the serializer's decision.
		got := SnakeToKebab(schemaAttr)
		if mapped, ok := drift[schemaAttr]; ok {
			got = SnakeToKebab(mapped)
		}

		if got != tc.want {
			t.Errorf("RouterOS %s: serializes %q as %q, want %q", tc.ros, schemaAttr, got, tc.want)
		}
	}
}
