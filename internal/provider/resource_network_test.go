package provider

import (
	"testing"
)

func TestNetworkResourceSchema(t *testing.T) {
	r := resourceComposeNetwork()

	if _, ok := r.Schema["name"]; !ok {
		t.Fatal("network resource schema missing 'name' field")
	}
	if !r.Schema["name"].Required {
		t.Error("network 'name' should be Required")
	}
	if !r.Schema["name"].ForceNew {
		t.Error("network 'name' should ForceNew")
	}

	optionalFields := []string{
		"driver", "driver_opts", "internal", "attachable",
		"labels", "ipam_driver", "ipam_subnet", "ipam_gateway",
	}
	for _, field := range optionalFields {
		s, ok := r.Schema[field]
		if !ok {
			t.Errorf("network schema missing field %q", field)
			continue
		}
		if s.Required {
			t.Errorf("network field %q should be Optional", field)
		}
		if !s.ForceNew {
			t.Errorf("network field %q should be ForceNew (Docker networks are immutable in place)", field)
		}
	}
}

func TestNetworkResourceHasNoUpdate(t *testing.T) {
	r := resourceComposeNetwork()
	if r.Update != nil {
		t.Error("network resource should have no Update func: all fields are ForceNew")
	}
	if r.Create == nil || r.Read == nil || r.Delete == nil {
		t.Error("network resource must define Create, Read, and Delete")
	}
}

