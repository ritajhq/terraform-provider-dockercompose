package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xRizur/terraform-provider-dockercompose/internal/docker"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceComposeNetwork defines a standalone Docker network, independent of any
// dockercompose_stack project. Use it when a network needs to be shared across
// multiple stacks: create it here, then join it from each stack's `network` block
// with external = true and external_name set to this resource's `name`.
func resourceComposeNetwork() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetworkCreate,
		Read:   resourceNetworkRead,
		Delete: resourceNetworkDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Literal Docker network name (not project-prefixed).",
			},
			"driver": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Network driver (bridge, overlay, host, none). Defaults to bridge.",
			},
			"driver_opts": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Driver-specific options.",
			},
			"internal": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Default:     false,
				Description: "Restrict external access.",
			},
			"attachable": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Default:     false,
				Description: "Allow manual container attachment (required for overlay networks joined by standalone containers).",
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Network labels.",
			},
			"ipam_driver": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "IPAM driver.",
			},
			"ipam_subnet": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "IPAM subnet (e.g. '172.28.0.0/16').",
			},
			"ipam_gateway": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "IPAM gateway (e.g. '172.28.0.1').",
			},
		},
	}
}

func resourceNetworkCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*docker.DockerClient)
	name := d.Get("name").(string)

	_, err := client.NetworkCreate(
		name,
		d.Get("driver").(string),
		toStrMap(d.Get("driver_opts").(map[string]interface{})),
		d.Get("internal").(bool),
		d.Get("attachable").(bool),
		toStrMap(d.Get("labels").(map[string]interface{})),
		d.Get("ipam_driver").(string),
		d.Get("ipam_subnet").(string),
		d.Get("ipam_gateway").(string),
	)
	if err != nil {
		return fmt.Errorf("error creating network %q: %s", name, err)
	}

	d.SetId(name)
	return resourceNetworkRead(d, m)
}

func resourceNetworkRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*docker.DockerClient)
	name := d.Id()

	out, err := client.NetworkInspect(name)
	if err != nil {
		// Network no longer exists (or engine unreachable for this name) → drop from state.
		if strings.Contains(err.Error(), "No such network") {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("error inspecting network %q: %s", name, err)
	}

	var inspected []struct {
		Name string `json:"Name"`
	}
	if err := json.Unmarshal([]byte(out), &inspected); err != nil {
		return fmt.Errorf("error parsing network inspect output for %q: %s", name, err)
	}
	if len(inspected) == 0 {
		d.SetId("")
		return nil
	}

	return nil
}

func resourceNetworkDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*docker.DockerClient)
	name := d.Id()

	if _, err := client.NetworkRemove(name); err != nil {
		if strings.Contains(err.Error(), "No such network") {
			return nil
		}
		return fmt.Errorf("error removing network %q: %s", name, err)
	}

	return nil
}
