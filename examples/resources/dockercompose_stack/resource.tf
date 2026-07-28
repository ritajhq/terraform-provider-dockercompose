terraform {
  required_providers {
    dockercompose = {
      source  = "xRizur/dockercompose"
      version = "~> 1.1"
    }
  }
}

resource "dockercompose_stack" "monitoring" {
  name = "metrics-stack"

  # Clean up associated volumes strictly on destroy
  remove_volumes_on_destroy = true

  # Primary database service
  service {
    name           = "database"
    image          = "postgres:15-alpine"
    container_name = "monitoring_db"
    restart        = "unless-stopped"

    environment = {
      POSTGRES_USER     = "admin"
      POSTGRES_PASSWORD = "secretpassword"
      POSTGRES_DB       = "metrics"
    }

    # Restrict Hardware resources
    resource_limits_memory = "512M"
    resource_limits_cpus   = "1.0"

    # Volume mounts are defined as strings (host:container or named:container)
    volumes = ["db_data:/var/lib/postgresql/data"]

    ports = ["5432:5432"]

    # Healthcheck attributes
    healthcheck_test     = ["CMD", "pg_isready", "-U", "admin"]
    healthcheck_interval = "10s"
    healthcheck_timeout  = "5s"
    healthcheck_retries  = 5

    networks = ["monitoring_net"]
  }

  # Web application
  service {
    name           = "grafana"
    image          = "grafana/grafana:latest"
    container_name = "monitoring_ui"
    depends_on     = ["database"]
    ports          = ["3000:3000"]

    volumes  = ["grafana_data:/var/lib/grafana"]
    networks = ["monitoring_net"]
  }

  # Register volume definitions
  volume {
    name = "db_data"
  }

  volume {
    name = "grafana_data"
  }

  # Register network definitions
  network {
    name = "monitoring_net"
  }
}

# Toggle this to enable "docker compose up --watch" for local dev, leave it
# false everywhere else (staging/prod/CI). watch is intended for local dev
# only: it starts a long-running, detached watcher process attached to this
# stack and is not meant for CI/prod applies.
variable "enable_watch" {
  type    = bool
  default = false
}

resource "dockercompose_stack" "webapp" {
  name  = "webapp-dev"
  watch = var.enable_watch

  service {
    name  = "web"
    image = "myapp:dev"
    ports = ["3000:3000"]

    volumes = ["./src:/app/src"]

    # develop.watch entries only take effect when watch = true above. With
    # watch = false, the exact same config runs as a plain `docker compose up -d`.
    develop_watch {
      path   = "./src"
      action = "sync"
      target = "/app/src"
      ignore = ["node_modules/", "*.log"]
    }

    develop_watch {
      path   = "./package.json"
      action = "rebuild"
    }
  }
}

# --- Provider-level defaults + per-resource overrides ---
#
# `watch` and `active_profiles` can both be set once on the provider and
# inherited by every dockercompose_stack, or overridden per-stack. The
# resource-level value always wins when explicitly set — including an
# explicit `false` or `[]` overriding a non-default provider setting.
provider "dockercompose" {
  alias = "dev"

  watch           = true
  active_profiles = ["debug"]
}

resource "dockercompose_stack" "inherits_provider_defaults" {
  provider = dockercompose.dev
  name     = "webapp-dev-defaults"

  # Neither `watch` nor `active_profiles` is set here, so this stack inherits
  # watch = true and active_profiles = ["debug"] from the provider above.
  service {
    name  = "web"
    image = "myapp:dev"
    ports = ["3001:3000"]

    develop_watch {
      path   = "./src"
      action = "sync"
      target = "/app/src"
    }
  }

  # Only starts because the "debug" profile is active (inherited from the provider).
  service {
    name     = "debugger"
    image    = "myapp:debug-tools"
    profiles = ["debug"]
  }
}

resource "dockercompose_stack" "overrides_provider_defaults" {
  provider = dockercompose.dev

  name = "webapp-ci"

  # Explicitly override both provider defaults for this stack: no watcher, no
  # profiles active, even though the provider default is watch = true and
  # active_profiles = ["debug"].
  watch           = false
  active_profiles = []

  service {
    name  = "web"
    image = "myapp:dev"
    ports = ["3002:3000"]
  }

  # Stays stopped here since active_profiles = [] overrides the provider default.
  service {
    name     = "debugger"
    image    = "myapp:debug-tools"
    profiles = ["debug"]
  }
}
