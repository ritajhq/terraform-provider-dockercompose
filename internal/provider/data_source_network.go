package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ritajhq/terraform-provider-dockercompose/internal/docker"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceComposeNetwork looks up an existing Docker network by name (created by
// dockercompose_network, plain `docker network create`, or another compose project)
// so its literal name and attributes can be referenced elsewhere — most commonly via
// external_name in a dockercompose_stack's network block, to join a network that
// this Terraform config doesn't itself create.
func dataSourceComposeNetwork() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceNetworkReadContext,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Docker network name to look up.",
			},
			"driver": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Network driver (bridge, overlay, host, none).",
			},
			"scope": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Network scope (local, swarm).",
			},
			"internal": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the network restricts external access.",
			},
			"attachable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether containers can be manually attached.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Network labels.",
			},
			"ipam_driver": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "IPAM driver.",
			},
			"ipam_subnet": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "IPAM subnet of the first configured pool (e.g. '172.28.0.0/16').",
			},
			"ipam_gateway": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "IPAM gateway of the first configured pool (e.g. '172.28.0.1').",
			},
		},
	}
}

// networkInspectEntry mirrors the fields of `docker network inspect` output that this
// data source exposes.
type networkInspectEntry struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Scope      string            `json:"Scope"`
	Internal   bool              `json:"Internal"`
	Attachable bool              `json:"Attachable"`
	Labels     map[string]string `json:"Labels"`
	IPAM       struct {
		Driver string `json:"Driver"`
		Config []struct {
			Subnet  string `json:"Subnet"`
			Gateway string `json:"Gateway"`
		} `json:"Config"`
	} `json:"IPAM"`
}

func dataSourceNetworkReadContext(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*docker.DockerClient)
	name := d.Get("name").(string)

	out, err := client.NetworkInspect(name)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error inspecting network %q: %s", name, err))
	}

	var inspected []networkInspectEntry
	if err := json.Unmarshal([]byte(out), &inspected); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing network inspect output for %q: %s", name, err))
	}
	if len(inspected) == 0 {
		return diag.Errorf("no network found named %q", name)
	}

	net := inspected[0]
	d.SetId(net.Name)

	if err := d.Set("driver", net.Driver); err != nil {
		return diag.FromErr(fmt.Errorf("error setting driver: %s", err))
	}
	if err := d.Set("scope", net.Scope); err != nil {
		return diag.FromErr(fmt.Errorf("error setting scope: %s", err))
	}
	if err := d.Set("internal", net.Internal); err != nil {
		return diag.FromErr(fmt.Errorf("error setting internal: %s", err))
	}
	if err := d.Set("attachable", net.Attachable); err != nil {
		return diag.FromErr(fmt.Errorf("error setting attachable: %s", err))
	}
	if err := d.Set("labels", net.Labels); err != nil {
		return diag.FromErr(fmt.Errorf("error setting labels: %s", err))
	}
	if err := d.Set("ipam_driver", net.IPAM.Driver); err != nil {
		return diag.FromErr(fmt.Errorf("error setting ipam_driver: %s", err))
	}

	if len(net.IPAM.Config) > 0 {
		if err := d.Set("ipam_subnet", net.IPAM.Config[0].Subnet); err != nil {
			return diag.FromErr(fmt.Errorf("error setting ipam_subnet: %s", err))
		}
		if err := d.Set("ipam_gateway", net.IPAM.Config[0].Gateway); err != nil {
			return diag.FromErr(fmt.Errorf("error setting ipam_gateway: %s", err))
		}
	}

	return nil
}
