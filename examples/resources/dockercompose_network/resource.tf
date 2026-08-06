terraform {
  required_providers {
    dockercompose = {
      source  = "ritajhq/dockercompose"
      version = "~> 1.1"
    }
  }
}

# --------------------------------------------------------------------------
# Shared network, owned independently of any single stack. Compose prefixes
# every network declared inside a stack's own `network` block with
# "<project>_", so two stacks can never join "the same" network just by
# declaring matching blocks - api-stack_edge_net and worker-stack_edge_net
# would be two separate networks. Creating it here, outside any compose
# project, means its name is never prefixed.
# --------------------------------------------------------------------------
resource "dockercompose_network" "edge" {
  name       = "edge_net"
  driver     = "bridge"
  attachable = true

  ipam_subnet  = "172.28.0.0/16"
  ipam_gateway = "172.28.0.1"
}

# --------------------------------------------------------------------------
# Platform stack: owns the backing services (Postgres, Redis) that other
# stacks depend on. Runs on the shared network so its containers are
# reachable by service name from the API and worker stacks below.
# --------------------------------------------------------------------------
resource "dockercompose_stack" "platform" {
  name = "platform-stack"

  remove_volumes_on_destroy = true

  service {
    name           = "postgres"
    image          = "postgres:15-alpine"
    container_name = "platform_postgres"
    restart        = "unless-stopped"

    environment = {
      POSTGRES_USER     = "app"
      POSTGRES_PASSWORD = "app-secret"
      POSTGRES_DB       = "app"
    }

    volumes  = ["pgdata:/var/lib/postgresql/data"]
    networks = ["edge"]

    healthcheck_test     = ["CMD", "pg_isready", "-U", "app"]
    healthcheck_interval = "10s"
    healthcheck_timeout  = "5s"
    healthcheck_retries  = 5
  }

  service {
    name           = "redis"
    image          = "redis:7-alpine"
    container_name = "platform_redis"
    restart        = "unless-stopped"
    networks       = ["edge"]
  }

  volume {
    name = "pgdata"
  }

  network {
    name          = "edge"
    external_name = dockercompose_network.edge.name
  }

  depends_on = [dockercompose_network.edge]
}

# --------------------------------------------------------------------------
# API stack: a separate project that talks to Postgres/Redis by their
# service names ("postgres", "redis") purely because it shares the same
# Docker network - no service discovery or hardcoded IPs required.
# --------------------------------------------------------------------------
resource "dockercompose_stack" "api" {
  name = "api-stack"

  service {
    name           = "api"
    image          = "myapp/api:latest"
    container_name = "api_web"
    restart        = "unless-stopped"
    ports          = ["8080:8080"]
    networks       = ["edge"]

    environment = {
      DATABASE_URL = "postgres://app:app-secret@postgres:5432/app"
      REDIS_URL    = "redis://redis:6379/0"
    }

    depends_on = ["worker"]
  }

  service {
    name           = "worker"
    image          = "myapp/worker:latest"
    container_name = "api_worker"
    restart        = "unless-stopped"
    networks       = ["edge"]

    environment = {
      DATABASE_URL = "postgres://app:app-secret@postgres:5432/app"
      REDIS_URL    = "redis://redis:6379/0"
    }
  }

  network {
    name          = "edge"
    external_name = dockercompose_network.edge.name
  }

  # Both dockercompose_network.edge (the network) and the platform stack
  # (Postgres/Redis) must exist before this stack starts, since its services
  # connect to them immediately on boot.
  depends_on = [dockercompose_network.edge, dockercompose_stack.platform]
}
