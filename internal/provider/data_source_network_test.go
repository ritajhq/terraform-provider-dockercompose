package provider

import (
	"testing"
)

func TestNetworkDataSourceSchema(t *testing.T) {
	ds := dataSourceComposeNetwork()

	if _, ok := ds.Schema["name"]; !ok {
		t.Fatal("network data source schema missing 'name' field")
	}
	if !ds.Schema["name"].Required {
		t.Error("network data source 'name' should be Required")
	}

	computedFields := []string{
		"driver", "scope", "internal", "attachable",
		"labels", "ipam_driver", "ipam_subnet", "ipam_gateway",
	}
	for _, field := range computedFields {
		s, ok := ds.Schema[field]
		if !ok {
			t.Errorf("network data source schema missing field %q", field)
			continue
		}
		if !s.Computed {
			t.Errorf("network data source field %q should be Computed", field)
		}
		if s.Required || s.Optional {
			t.Errorf("network data source field %q should be read-only (not Required/Optional)", field)
		}
	}
}

func TestNetworkDataSourceHasReadContext(t *testing.T) {
	ds := dataSourceComposeNetwork()
	if ds.ReadContext == nil {
		t.Error("network data source must define ReadContext")
	}
}
