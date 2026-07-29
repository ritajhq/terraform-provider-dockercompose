terraform {
  required_providers {
    dockercompose = {
      source  = "xRizur/dockercompose"
      version = "~> 1.1"
    }
  }
}

# A network managed independently of any single stack, so it can be shared
# across multiple dockercompose_stack resources without either one owning
# (and prefixing) it.
resource "dockercompose_network" "shared" {
  name       = "shared_net"
  driver     = "bridge"
  attachable = true

  ipam_subnet  = "172.28.0.0/16"
  ipam_gateway = "172.28.0.1"
}

# Each stack joins the shared network by declaring it as external and
# pointing external_name at the literal Docker network name above. Without
# external_name, Compose would prefix the network with "<project>_" and each
# stack would end up with its own private network of the same short name.
resource "dockercompose_stack" "api" {
  name = "api-stack"

  service {
    name     = "api"
    image    = "myapp/api:latest"
    networks = ["shared_net"]
  }

  network {
    name          = "shared_net"
    external      = true
    external_name = dockercompose_network.shared.name
  }

  depends_on = [dockercompose_network.shared]
}

resource "dockercompose_stack" "worker" {
  name = "worker-stack"

  service {
    name     = "worker"
    image    = "myapp/worker:latest"
    networks = ["shared_net"]
  }

  network {
    name          = "shared_net"
    external      = true
    external_name = dockercompose_network.shared.name
  }

  depends_on = [dockercompose_network.shared]
}
