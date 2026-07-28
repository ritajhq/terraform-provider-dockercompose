package provider

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
)

// ============================================================
// Unit Tests for resolve.go: provider-default vs resource-override resolution
// ============================================================

func TestAttrConfiguredNilConfig(t *testing.T) {
	if attrConfigured(cty.NilVal, "watch") {
		t.Error("attrConfigured(NilVal) = true, want false")
	}
}

func TestAttrConfiguredMissingAttribute(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{
		"name": cty.StringVal("stack1"),
	})
	if attrConfigured(obj, "watch") {
		t.Error("attrConfigured() = true for an attribute the object type doesn't have, want false")
	}
}

func TestAttrConfiguredUnsetBool(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{
		"watch": cty.NullVal(cty.Bool),
	})
	if attrConfigured(obj, "watch") {
		t.Error("attrConfigured() = true for a null (unset) bool attribute, want false")
	}
}

func TestAttrConfiguredExplicitFalse(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{
		"watch": cty.False,
	})
	if !attrConfigured(obj, "watch") {
		t.Error("attrConfigured() = false for an explicit `false` value, want true")
	}
}

func TestAttrConfiguredExplicitTrue(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{
		"watch": cty.True,
	})
	if !attrConfigured(obj, "watch") {
		t.Error("attrConfigured() = false for an explicit `true` value, want true")
	}
}

func TestAttrConfiguredUnsetList(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{
		"active_profiles": cty.NullVal(cty.List(cty.String)),
	})
	if attrConfigured(obj, "active_profiles") {
		t.Error("attrConfigured() = true for a null (unset) list attribute, want false")
	}
}

func TestAttrConfiguredExplicitEmptyList(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{
		"active_profiles": cty.ListValEmpty(cty.String),
	})
	if !attrConfigured(obj, "active_profiles") {
		t.Error("attrConfigured() = false for an explicit empty list, want true (empty is not the same as unset)")
	}
}

func TestAttrConfiguredExplicitNonEmptyList(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{
		"active_profiles": cty.ListVal([]cty.Value{cty.StringVal("dev")}),
	})
	if !attrConfigured(obj, "active_profiles") {
		t.Error("attrConfigured() = false for an explicit non-empty list, want true")
	}
}

func TestResolveBoolValueInheritsProviderDefault(t *testing.T) {
	// Not configured on the resource: provider default (true) wins, regardless of
	// whatever zero-value happens to be sitting in resourceValue.
	got := resolveBoolValue(false, false, true)
	if got != true {
		t.Errorf("resolveBoolValue(configured=false, resourceValue=false, providerDefault=true) = %v, want true", got)
	}
}

func TestResolveBoolValueResourceOverrideTrue(t *testing.T) {
	got := resolveBoolValue(true, true, false)
	if got != true {
		t.Errorf("resolveBoolValue(configured=true, resourceValue=true, providerDefault=false) = %v, want true", got)
	}
}

func TestResolveBoolValueResourceOverrideFalse(t *testing.T) {
	// The critical three-state case: an explicit `false` on the resource must win
	// over a provider default of `true`.
	got := resolveBoolValue(true, false, true)
	if got != false {
		t.Errorf("resolveBoolValue(configured=true, resourceValue=false, providerDefault=true) = %v, want false", got)
	}
}

func TestResolveStringListValueInheritsProviderDefault(t *testing.T) {
	def := []string{"dev", "debug"}
	got := resolveStringListValue(false, []string{"ignored"}, def)
	if len(got) != 2 || got[0] != "dev" || got[1] != "debug" {
		t.Errorf("resolveStringListValue(configured=false, ...) = %v, want provider default %v", got, def)
	}
}

func TestResolveStringListValueResourceOverride(t *testing.T) {
	got := resolveStringListValue(true, []string{"prod"}, []string{"dev"})
	if len(got) != 1 || got[0] != "prod" {
		t.Errorf("resolveStringListValue(configured=true, ...) = %v, want [prod]", got)
	}
}

func TestResolveStringListValueExplicitEmptyOverride(t *testing.T) {
	// An explicit empty list on the resource must override a non-empty provider
	// default, clearing all active profiles for this stack.
	got := resolveStringListValue(true, []string{}, []string{"dev", "debug"})
	if len(got) != 0 {
		t.Errorf("resolveStringListValue(configured=true, resourceValue=[]) = %v, want empty", got)
	}
}
