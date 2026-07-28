package provider

import (
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// attrConfigured reports whether key was explicitly set in raw (the resource's raw
// HCL config), as opposed to being entirely omitted. This is the piece that plain
// Get/GetOk can't provide reliably for bools (false vs. unset look identical) or for
// lists (GetOk treats an explicit empty list the same as "not set").
func attrConfigured(raw cty.Value, key string) bool {
	if raw.IsNull() {
		return false
	}
	if !raw.Type().HasAttribute(key) {
		return false
	}
	return !raw.GetAttr(key).IsNull()
}

// resolveBoolValue is the pure decision function behind resolveBool: if configured,
// the resource-level value wins (including explicit false); otherwise providerDefault.
func resolveBoolValue(configured bool, resourceValue, providerDefault bool) bool {
	if configured {
		return resourceValue
	}
	return providerDefault
}

// resolveStringListValue is the pure decision function behind resolveStringList: if
// configured, the resource-level value wins (including an explicit empty list);
// otherwise providerDefault.
func resolveStringListValue(configured bool, resourceValue, providerDefault []string) []string {
	if configured {
		return resourceValue
	}
	return providerDefault
}

// resolveBool returns the resource-level value for key if it was explicitly set in
// the resource's config, otherwise falls back to providerDefault. Using GetRawConfig
// (rather than GetOk/GetOkExists) is what lets an explicit `false` on the resource
// override a provider-level default of `true` — a plain Get can't distinguish
// "explicitly false" from "not set" for a bool attribute.
func resolveBool(d *schema.ResourceData, key string, providerDefault bool) bool {
	configured := attrConfigured(d.GetRawConfig(), key)
	return resolveBoolValue(configured, d.Get(key).(bool), providerDefault)
}

// resolveStringList returns the resource-level value for key if it was explicitly
// set in the resource's config (including an explicit empty list), otherwise falls
// back to providerDefault. GetRawConfig is used instead of GetOk because GetOk
// treats an explicitly-empty list the same as "not set", which would prevent a
// resource from ever overriding a non-empty provider default with an empty list.
func resolveStringList(d *schema.ResourceData, key string, providerDefault []string) []string {
	configured := attrConfigured(d.GetRawConfig(), key)
	resourceValue := getStrListValue(d.Get(key).([]interface{}))
	return resolveStringListValue(configured, resourceValue, providerDefault)
}
