terraform {
  required_providers {
    dockercompose = {
      source  = "ritajhq/dockercompose"
      version = "~> 1.1"
    }
  }
}

# Look up a network this Terraform config doesn't manage — e.g. "edge" was
# created outside of Terraform (by another team, a bootstrap script, or
# `docker network create` directly). Fails at plan/apply time if it doesn't
# exist, instead of silently creating a new project-prefixed network.
data "dockercompose_network" "edge" {
  name = "edge"
}

# Join it from a stack by pointing external_name at the looked-up name.
# external must still be true: this stack does not own the network's
# lifecycle, it only attaches to it.
resource "dockercompose_stack" "api" {
  name = "api-stack"

  service {
    name     = "api"
    image    = "myapp/api:latest"
    networks = ["edge"]
  }

  network {
    name          = "edge"
    external      = true
    external_name = data.dockercompose_network.edge.name
  }
}

output "edge_driver" {
  description = "Driver of the looked-up edge network"
  value       = data.dockercompose_network.edge.driver
}

output "edge_subnet" {
  description = "IPAM subnet of the edge network, if any"
  value       = data.dockercompose_network.edge.ipam_subnet
}
