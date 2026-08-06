# Terraform Provider for Docker Compose

[![Terraform Registry](https://img.shields.io/badge/registry-ritajhq%2Fdockercompose-844FBA?logo=terraform)](https://registry.terraform.io/providers/ritajhq/dockercompose/latest)
[![Release](https://img.shields.io/github/v/release/ritajhq/terraform-provider-dockercompose?label=release)](https://github.com/ritajhq/terraform-provider-dockercompose/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ritajhq/terraform-provider-dockercompose)](go.mod)
[![Stars](https://img.shields.io/github/stars/ritajhq/terraform-provider-dockercompose?style=social)](https://github.com/ritajhq/terraform-provider-dockercompose/stargazers)

> Manage Docker Compose stacks as Terraform resources. Define multi-container applications in HCL, deploy them to local or remote Docker hosts via SSH/TCP/Unix socket, and get full lifecycle management with comprehensive Docker Compose v3 spec coverage.

## Why this provider?

The official Docker provider (`kreuzwerker/docker`) treats every container, network, and volume as a separate Terraform resource. That works, but it loses the Docker Compose mental model — you end up reimplementing service dependencies, healthcheck wait logic, and compose-style lifecycle by hand.

This provider takes the opposite approach: **a stack is one resource**. You write HCL that mirrors `docker-compose.yml` 1:1, and the provider runs `docker compose up -d` under the hood — preserving Compose's dependency graph, healthcheck-aware startup, and project isolation.

| | `ritajhq/dockercompose` | `kreuzwerker/docker` |
|---|---|---|
| Mental model | Compose stacks | Individual containers |
| Service dependencies | `depends_on` (Compose-native) | Manual `depends_on` between TF resources |
| Healthcheck-aware startup | Native (Compose handles it) | Manual provisioner workarounds |
| Existing `docker-compose.yml` reuse | Drop-in via `dockercompose_project` | Rewrite as TF resources |
| Lines of HCL for 5-service stack | ~50 | ~200+ |
| Remote hosts (SSH/TCP) | Yes | Yes |

**Pick this provider when:** you already think in Compose, you want to migrate `docker-compose.yml` files into IaC without rewriting them, or you're managing self-hosted apps / homelab / dev environments where Compose is the natural unit of deployment.

## Use cases

- **Homelab and self-hosted apps** — manage your `*arr` stack, Nextcloud, Vaultwarden, Pi-hole etc. as code, with version-pinned image tags and reproducible deployments.
- **Edge / single-node production** — deploy to a remote VPS over SSH (`ssh://deploy@server`) without installing Terraform, Docker Swarm, or Kubernetes on the box.
- **Ephemeral CI/CD environments** — spin up full app stacks per branch or PR, tear them down on merge.
- **Dev environments as code** — give every developer a one-command (`terraform apply`) local stack that matches production.
- **Migrating legacy Compose files** — point `dockercompose_project` at an existing `docker-compose.yml` and you're done; no rewrite required.

## Features

- **Remote host support** - connect via SSH, TCP, or Unix socket (like the Docker provider)
- **Three resource types**:
  - `dockercompose_stack` - full HCL-modeled services, networks, volumes, configs, secrets
  - `dockercompose_project` - use existing `docker-compose.yml` files or inline YAML
  - `dockercompose_network` - standalone Docker network shared across multiple stacks, unaffected by Compose's per-project name prefixing
- **Comprehensive service config** - ports, volumes, environment, healthchecks, deploy resources, logging, security options, sysctls, devices, and 50+ other Docker Compose fields
- **Network & volume management** - drivers, IPAM, external references, labels, driver options
- **Docker configs & secrets** - top-level config/secret definitions
- **Project isolation** - each stack gets its own project name (`-p`) and directory
- **State management** - generated YAML stored in Terraform state, auto-restored if deleted from disk
- **Import support** - import existing stacks with `terraform import`
- **Compose Watch support** - `watch = true` runs `docker compose up -d --watch` as a detached process for live file sync during local dev, driven by `develop_watch` blocks on each service
- **Compose profiles** - `active_profiles` activates `docker compose --profile` for services tagged with `profiles`, so profile-scoped services actually start/stop
- **Provider-level defaults** - `watch` and `active_profiles` can be set once on the provider and inherited by every `dockercompose_stack`, with per-resource overrides

## Quick Start

```hcl
terraform {
  required_providers {
    dockercompose = {
      source = "ritajhq/dockercompose"
    }
  }
}

provider "dockercompose" {}

resource "dockercompose_stack" "app" {
  name = "myapp"

  service {
    name    = "web"
    image   = "nginx:alpine"
    restart = "unless-stopped"
    ports   = ["8080:80"]
  }

  service {
    name    = "db"
    image   = "postgres:17-alpine"
    restart = "always"
    ports   = ["5432:5432"]
    volumes = ["pgdata:/var/lib/postgresql/data"]
    environment = {
      POSTGRES_PASSWORD = "secret"
    }
    healthcheck_test     = ["CMD-SHELL", "pg_isready"]
    healthcheck_interval = "10s"
  }

  volume {
    name = "pgdata"
  }
}
```

## Provider Configuration

```hcl
provider "dockercompose" {
  # Connect to remote Docker host (optional)
  host = "ssh://deploy@production-server"
  # host = "tcp://docker.example.com:2376"
  # host = "unix:///var/run/docker.sock"

  # Custom docker binary path (default: "docker")
  docker_binary = "/usr/local/bin/docker"

  # Directory for generated compose files (default: ~/.terraform-docker-compose)
  project_directory = "/opt/compose-projects"

  # Default for dockercompose_stack's `watch` attribute (default: false).
  # Individual stacks can override this by setting `watch` explicitly.
  watch = false

  # Default for dockercompose_stack's `active_profiles` attribute (default: none).
  # Individual stacks can override this by setting `active_profiles` explicitly
  # (including [] to force no profiles active).
  active_profiles = []
}
```

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `host` | string | `$DOCKER_HOST` | Docker daemon URL (ssh://, tcp://, unix://) |
| `docker_binary` | string | `"docker"` | Path to docker binary |
| `project_directory` | string | `~/.terraform-docker-compose` | Base directory for compose files |
| `watch` | bool | `false` | Provider-level default for `dockercompose_stack`'s `watch` attribute |
| `active_profiles` | list(string) | `[]` | Provider-level default for `dockercompose_stack`'s `active_profiles` attribute |

`watch` and `active_profiles` follow a "provider default, resource-level override" pattern: if a `dockercompose_stack` resource sets the attribute explicitly (including `watch = false` or `active_profiles = []`), that value wins over the provider default; otherwise the resource inherits the provider's value. See [Watch (live file sync)](#watch-live-file-sync) and [Compose Profiles](#compose-profiles) below.

## Resources

### `dockercompose_stack`

Full HCL-modeled Docker Compose stack with typed attributes.

#### Top-level Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Project name (ForceNew) |
| `working_dir` | string | no | Working directory for relative paths |
| `remove_volumes_on_destroy` | bool | no | Run `docker compose down -v` on destroy |
| `watch` | bool | no | Launch `docker compose up -d --watch` instead of a plain `up -d`. Defaults to the provider-level `watch` setting; set explicitly (including `false`) to override it. See [Watch](#watch-live-file-sync). |
| `active_profiles` | list(string) | no | Docker Compose profiles to activate. Defaults to the provider-level `active_profiles` setting; set explicitly (including `[]`) to override it. See [Compose Profiles](#compose-profiles). |

#### Computed Outputs

| Attribute | Description |
|-----------|-------------|
| `compose_yaml` | Generated YAML content |
| `compose_file_path` | Path to generated file |
| `container` | List of container runtime info (see below) |
| `watch_pid` | PID of the running `docker compose --watch` process, if active. `0`/unset if watch is disabled or the watcher isn't running. |

#### Container Block (computed)

After `apply`, every running container is exposed as a computed `container` list sorted by service name.
Use `for` expressions to look up specific services.

| Attribute | Type | Description |
|-----------|------|-------------|
| `service` | string | Service name |
| `container_id` | string | Short container ID (12 chars) |
| `container_name` | string | Container name (e.g. `myapp-web-1`) |
| `image` | string | Docker image |
| `state` | string | `running`, `exited`, `paused`, etc. |
| `health` | string | `healthy`, `unhealthy`, `starting`, or empty |
| `exit_code` | int | Exit code (0 when running) |
| `ip_address` | string | IP on the first attached network |
| `ports` | list | Published port mappings (see sub-table) |
| `network_settings` | list | Per-network details (see sub-table) |

**`ports` sub-attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `ip` | string | Bound host IP (e.g. `0.0.0.0`) |
| `private_port` | int | Container-side port |
| `public_port` | int | Host-side port (0 if expose-only) |
| `protocol` | string | `tcp` or `udp` |

**`network_settings` sub-attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `name` | string | Docker network name |
| `ip_address` | string | Container IP on this network |
| `gateway` | string | Network gateway |
| `mac_address` | string | Container MAC address |

**Example outputs:**

```hcl
# Single service lookup
output "web_ip" {
  value = [for c in dockercompose_stack.app.container : c.ip_address if c.service == "web"][0]
}

# All IPs as a map
output "container_ips" {
  value = { for c in dockercompose_stack.app.container : c.service => c.ip_address }
}

# Published ports
output "published_ports" {
  value = {
    for c in dockercompose_stack.app.container : c.service => [
      for p in c.ports : "${p.ip}:${p.public_port}->${p.private_port}/${p.protocol}"
      if p.public_port > 0
    ] if length([for p in c.ports : p if p.public_port > 0]) > 0
  }
}

# Network details for a specific service
output "db_networks" {
  value = [for c in dockercompose_stack.app.container : c.network_settings if c.service == "db"][0]
}

# Container ID (useful with docker_exec or provisioners)
output "web_container_id" {
  value = [for c in dockercompose_stack.app.container : c.container_id if c.service == "web"][0]
}
```

#### Service Block

```hcl
service {
  # Core
  name           = "api"
  image          = "myapp:latest"
  container_name = "myapp-api"
  restart        = "unless-stopped"    # no, always, on-failure, unless-stopped
  ports          = ["3000:3000"]
  expose         = ["9090"]
  depends_on     = ["db", "redis"]
  environment    = { NODE_ENV = "production" }
  env_file       = [".env", ".env.production"]
  command        = ["node", "server.js"]
  entrypoint     = ["/entrypoint.sh"]
  volumes        = ["./data:/data", "dbvol:/db"]
  networks       = ["frontend", "backend"]
  labels         = { "com.example.team" = "backend" }

  # Deploy
  replicas                     = 3
  resource_limits_cpus         = "0.5"
  resource_limits_memory       = "512M"
  resource_reservations_cpus   = "0.25"
  resource_reservations_memory = "256M"

  # Healthcheck
  healthcheck_test         = ["CMD", "curl", "-f", "http://localhost:3000/health"]
  healthcheck_interval     = "30s"
  healthcheck_timeout      = "10s"
  healthcheck_retries      = 3
  healthcheck_start_period = "40s"
  healthcheck_disable      = false

  # Logging
  logging_driver  = "json-file"
  logging_options  = { "max-size" = "10m", "max-file" = "3" }

  # Security
  cap_add      = ["NET_ADMIN"]
  cap_drop     = ["ALL"]
  security_opt = ["no-new-privileges:true"]
  privileged   = false
  read_only    = true
  init         = true
  user         = "1000:1000"
  group_add    = ["1001", "shared-group"]

  # Networking
  dns          = ["8.8.8.8"]
  extra_hosts  = ["host.docker.internal:host-gateway"]
  hostname     = "api"
  domainname   = "example.com"
  network_mode = ""         # bridge, host, none, service:name

  # Runtime
  working_dir       = "/app"
  stdin_open        = false
  tty               = false
  shm_size          = "256m"
  stop_grace_period = "30s"
  stop_signal       = "SIGTERM"
  platform          = "linux/amd64"
  pull_policy       = "always"    # always, never, missing, build
  runtime           = "runc"
  tmpfs             = ["/tmp"]
  devices           = ["/dev/sda:/dev/xvdc:rwm"]
  sysctls           = { "net.core.somaxconn" = "1024" }
  profiles          = ["debug"]
  pid               = ""
  ipc               = ""

  # Legacy resource limits
  mem_limit       = ""
  mem_reservation = ""
  cpus            = ""

  # Compose Watch (develop.watch) — only takes effect when the stack's
  # resolved `watch` is true. See Watch (live file sync) below.
  develop_watch {
    path   = "./src"
    action = "sync"           # sync, rebuild, sync+restart, sync+exec
    target = "/app/src"
    ignore = ["node_modules/", "*.log"]
  }

  develop_watch {
    path   = "./package.json"
    action = "rebuild"
  }
}
```

#### Network Block

```hcl
network {
  name        = "backend"
  driver      = "bridge"
  driver_opts = { "com.docker.network.bridge.enable_icc" = "true" }
  external    = false
  internal    = true
  attachable  = false
  labels      = { "env" = "production" }

  # IPAM
  ipam_driver  = "default"
  ipam_subnet  = "172.28.0.0/16"
  ipam_gateway = "172.28.0.1"
}
```

#### Volume Block

```hcl
volume {
  name        = "dbdata"
  driver      = "local"
  driver_opts = { "type" = "nfs", "o" = "addr=192.168.1.1", "device" = ":/data" }
  external    = false
  labels      = { "backup" = "daily" }
}
```

#### Config & Secret Blocks

```hcl
config {
  name = "nginx_config"
  file = "./nginx.conf"
}

secret {
  name = "db_password"
  file = "./secrets/db_pass.txt"
}
```

#### Watch (live file sync)

`watch = true` launches `docker compose up -d --watch` as a **detached, long-running background process** instead of a plain `up -d`, syncing local file changes into running containers per each service's `develop_watch` blocks. It's intended for **local dev only** — not CI/prod applies.

```hcl
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

    develop_watch {
      path   = "./src"
      action = "sync"
      target = "/app/src"
    }
  }
}
```

- If `watch = true` but no service defines any `develop_watch` entries, this silently falls back to a plain `up -d` (no watcher, no warning).
- The watcher's stdout/stderr are redirected to `<project_directory>/<name>/watch.log`, and its PID is written to `<project_directory>/<name>/watch.pid` as well as the computed `watch_pid` attribute.
- **Read** checks whether the recorded PID is still alive; if it has died, `watch_pid` is cleared so the drift is visible (no error).
- **Update** always kills and respawns the watcher if one is (or should be) running, regardless of whether the compose diff itself required it.
- **Delete** sends SIGTERM (then SIGKILL after a grace period) to the watcher before tearing down the stack. If the PID is already dead, this logs a warning and proceeds — destroy never fails because of a dead watcher.
- `watch` can also be set once on the `provider` block and inherited by every stack — see [Provider-level defaults](#provider-level-defaults-watch--active_profiles) below.

#### Compose Profiles

`active_profiles` activates [Docker Compose profiles](https://docs.docker.com/compose/how-tos/profiles/) for a stack, so services tagged with a matching `profiles = [...]` on their service block actually start (rather than being defined but left dormant).

```hcl
resource "dockercompose_stack" "app" {
  name            = "myapp"
  active_profiles = ["debug"]

  service {
    name  = "web"
    image = "myapp:latest"
  }

  # Only starts because "debug" is in active_profiles above.
  service {
    name     = "debugger"
    image    = "myapp:debug-tools"
    profiles = ["debug"]
  }
}
```

- A service with no `profiles` set is always active.
- On **Update**, the previously-applied `active_profiles` (stored in state) is diffed against the newly resolved value. Services whose profile dropped out are stopped (`docker compose stop <service>`, not removed) before re-applying with the new `--profile` flags.
- **Read** scopes both the running-services check and the computed `container` list to the active profile set.
- `active_profiles` can also be set once on the `provider` block and inherited by every stack — see below.

#### Provider-level defaults (`watch` / `active_profiles`)

Both attributes follow the same **provider default, resource-level override** pattern: if the resource sets the attribute explicitly, that value wins — including an explicit `watch = false` overriding a provider default of `true`, or `active_profiles = []` overriding a non-empty provider default. Otherwise the resource inherits the provider's value.

```hcl
provider "dockercompose" {
  alias = "dev"

  watch           = true
  active_profiles = ["debug"]
}

resource "dockercompose_stack" "inherits_defaults" {
  provider = dockercompose.dev
  name     = "webapp-dev-defaults"

  # Neither `watch` nor `active_profiles` set: inherits watch = true and
  # active_profiles = ["debug"] from the provider above.
  service {
    name  = "web"
    image = "myapp:dev"
    develop_watch {
      path   = "./src"
      action = "sync"
      target = "/app/src"
    }
  }
}

resource "dockercompose_stack" "overrides_defaults" {
  provider = dockercompose.dev
  name     = "webapp-ci"

  # Explicit overrides win over the provider defaults above.
  watch           = false
  active_profiles = []

  service {
    name  = "web"
    image = "myapp:dev"
  }
}
```

This lets a single stack configuration be reused across local dev (`watch = true`) and other environments (`watch = false`) just by toggling a variable or provider default — see the [full example](examples/resources/dockercompose_stack/resource.tf).

### `dockercompose_project`

Manages a stack from an existing `docker-compose.yml` file or inline YAML.

```hcl
# From file
resource "dockercompose_project" "legacy" {
  name         = "legacy-app"
  compose_file = "${path.module}/docker-compose.yml"
}

# Inline YAML (supports templatefile)
resource "dockercompose_project" "dynamic" {
  name = "dynamic-app"
  compose_yaml = templatefile("${path.module}/compose.yml.tpl", {
    image_tag   = var.image_tag
    db_password = var.db_password
  })
}
```

| Attribute | Type | Description |
|-----------|------|-------------|
| `name` | string | Project name (required, ForceNew) |
| `compose_file` | string | Path to compose file (conflicts with compose_yaml) |
| `compose_yaml` | string | Inline YAML content (conflicts with compose_file) |
| `remove_volumes_on_destroy` | bool | Remove volumes on destroy |
| `yaml_sha256` | string | (computed) SHA256 of YAML content |
| `container` | list | (computed) Container runtime info - same schema as `dockercompose_stack` |

### `dockercompose_network`

A standalone Docker network (`docker network create`), independent of any `dockercompose_stack`'s compose project. Use it to share a network across multiple stacks — Compose prefixes every network declared inside a stack's `network` block with `<project>_`, so two stacks can't join "the same" network just by declaring matching blocks. Create it here instead, then have each stack join it by pointing `external_name` at this resource's `name` to pin the literal Docker network name (this implies `external = true`, no need to set that separately).

```hcl
resource "dockercompose_network" "shared" {
  name       = "shared_net"
  driver     = "bridge"
  attachable = true
}

resource "dockercompose_stack" "api" {
  name = "api-stack"

  service {
    name     = "api"
    image    = "myapp/api:latest"
    networks = ["shared_net"]
  }

  network {
    name          = "shared_net"
    external_name = dockercompose_network.shared.name
  }

  depends_on = [dockercompose_network.shared]
}
```

Prefer to reference a network you don't manage in Terraform at all (created by another team, or by hand)? Use the `dockercompose_network` **data source** instead of the resource:

```hcl
data "dockercompose_network" "edge" {
  name = "edge"
}

resource "dockercompose_stack" "api" {
  name = "api-stack"

  service {
    name     = "api"
    image    = "myapp/api:latest"
    networks = ["edge"]
  }

  network {
    name          = "edge"
    external_name = data.dockercompose_network.edge.name
  }
}
```

The data source exposes `driver`, `scope`, `internal`, `attachable`, `labels`, `ipam_driver`, `ipam_subnet`, `ipam_gateway` as computed attributes, and fails at plan/apply time if the named network doesn't exist.

| Attribute | Type | Description |
|-----------|------|-------------|
| `name` | string | Literal Docker network name, not project-prefixed (required, ForceNew) |
| `driver` | string | Network driver: bridge, overlay, host, none (ForceNew) |
| `driver_opts` | map(string) | Driver-specific options (ForceNew) |
| `internal` | bool | Restrict external access (ForceNew) |
| `attachable` | bool | Allow manual container attachment (ForceNew) |
| `labels` | map(string) | Network labels (ForceNew) |
| `ipam_driver` | string | IPAM driver (ForceNew) |
| `ipam_subnet` | string | IPAM subnet, e.g. `172.28.0.0/16` (ForceNew) |
| `ipam_gateway` | string | IPAM gateway, e.g. `172.28.0.1` (ForceNew) |

All attributes are `ForceNew` — Docker networks can't be modified in place, so any change destroys and recreates the network. Destroying it while a stack still references it externally fails at the engine level ("network has active endpoints"); add `depends_on = [dockercompose_network.x]` on each referencing stack so Terraform tears stacks down first.

See [`examples/resources/dockercompose_network/`](examples/resources/dockercompose_network/) for a fuller example: a shared network, a "platform" stack owning Postgres/Redis, and a separate API/worker stack that reaches them purely by service name.

## How It Works

1. **Create/Update**: Builds YAML from HCL config (or uses provided YAML), writes to `<project_directory>/<name>/docker-compose.yml`. Resolves `watch`/`active_profiles` (resource override, else provider default) and runs `docker compose -p <name> up -d --remove-orphans [--profile ...]` — or, if watch is resolved to `true` and at least one service has `develop_watch` entries, launches a detached `docker compose -p <name> up -d --watch [--profile ...]` instead. On Update, any previously-dropped `active_profiles` are stopped first, and a running watcher is always killed and respawned.
2. **Read**: Checks if compose file exists (restores from state if missing), verifies services are running via `docker compose ps --services` scoped to the active profiles, and self-heals `watch_pid` if the recorded watcher process has died.
3. **Delete**: Stops any running watcher (SIGTERM, then SIGKILL after a grace period; a warning if already dead), runs `docker compose -p <name> down [-v]`, cleans up generated files.
4. **State**: Generated YAML, resolved `watch`/`active_profiles`, and `watch_pid` are stored in Terraform state for recovery and drift detection. Each stack isolated by project name and directory.

## Architecture

```
main.go                    Entry point
provider.go                Provider schema + configuration (host, binary, directory, watch, active_profiles)
docker.go                  DockerClient - CLI wrapper with DOCKER_HOST support, --profile flags, detached watch process
compose.go                 ComposeFile structs + YAML marshal/unmarshal (incl. develop.watch)
container_info.go          Container runtime schema, JSON parsing, readContainerInfo
resource_stack.go          dockercompose_stack - HCL-to-YAML resource
resource_project.go        dockercompose_project - raw YAML/file resource
resolve.go                 Provider-default vs resource-override resolution (watch, active_profiles)
watch_lifecycle.go         Watch process start/reconcile/stop shared between stack and project
profile_lifecycle.go       active_profiles diffing (which services to stop when a profile drops)
utils.go                   Type-safe extraction helpers
```

## Building

The provider requires Go 1.24+ to build. If you don't have Go installed, `scripts/build.sh` builds it in Docker instead:

```bash
./scripts/build.sh
# -> ./dist/terraform-provider-dockercompose
```

It builds a throwaway image from `Dockerfile.build`, `docker cp`s the compiled binary out of a container created from it, and removes the container — nothing but the binary is left behind.

With Go installed locally:

```bash
go build -o terraform-provider-dockercompose      # Linux / macOS
go build -o terraform-provider-dockercompose.exe   # Windows
```

## Local Testing (dev override)

The fastest way to test the provider locally - no registry, no `terraform init`.

### Prerequisites

- **Go** >= 1.21 (or just Docker — see below)
- **Terraform** >= 1.0
- **Docker Desktop** (or Docker Engine with the Compose plugin)
- **Windows only**: Developer Mode enabled (Settings → System → For developers)

### 1. Build the provider binary

With Go installed:

```bash
# From the repository root
go build -o terraform-provider-dockercompose.exe .   # Windows
go build -o terraform-provider-dockercompose .        # Linux / macOS
```

Without Go installed, build it in Docker instead:

```bash
./scripts/build.sh
# -> ./dist/terraform-provider-dockercompose
```

### 2. Create the Terraform CLI config with dev_overrides

**Windows** - create `%APPDATA%\terraform.rc` (typically `C:\Users\<you>\AppData\Roaming\terraform.rc`):

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/ritajhq/dockercompose" = "C:\\Users\\<you>\\path\\to\\DockerCompose-Terraform-Provider"
  }
  direct {}
}
```

**Linux / macOS** - create `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/ritajhq/dockercompose" = "/home/<you>/path/to/DockerCompose-Terraform-Provider"
  }
  direct {}
}
```

> **The path must point to the directory *containing* the built binary, not the binary file itself.** If you built with `scripts/build.sh`, point this at the `dist/` folder (e.g. `/home/<you>/path/to/repo/dist`), not `dist/terraform-provider-dockercompose`. Pointing at the binary file directly produces an error like `could not read package directory: ... not a directory`.

### 3. Write a Terraform config

```hcl
terraform {
  required_providers {
    dockercompose = {
      source = "ritajhq/dockercompose"
    }
  }
}

provider "dockercompose" {}

resource "dockercompose_stack" "demo" {
  name = "demo"
  service {
    name  = "web"
    image = "nginx:alpine"
    ports = ["9090:80"]
  }
}
```

### 4. Run Terraform

```bash
# No 'terraform init' needed with dev_overrides!
terraform plan
terraform apply -auto-approve

# Verify
docker compose -p demo ps

# Clean up
terraform destroy -auto-approve
```

> The warning "Provider development overrides are in effect" is expected - it means Terraform is using your local build.

### 5. Running the test suite

```bash
# Unit + integration tests (no Docker required)
go test -v -run "Test[^A]" -count=1 ./...

# Full suite including acceptance tests (Docker required)
TF_ACC=1 go test -v -count=1 -timeout 10m ./...        # Linux / macOS
$env:TF_ACC = "1"; go test -v -count=1 -timeout 10m ./...  # Windows PowerShell

# Coverage report (opens in browser)
go test -run "Test[^A]" -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Requirements

- Docker Engine with the Compose plugin (`docker compose`)
- Go 1.24+ (for building) — or just Docker, via `./scripts/build.sh`
- For remote hosts: SSH access configured or TLS certificates

## Breaking Changes from v1

- **Provider config**: New `host`, `docker_binary`, `project_directory` fields
- **Service block**: Changed from `TypeSet` to `TypeList` - service order in HCL matters
- **Healthcheck**: `healthcheck_test` is now a list (was a string)
- **Replicas**: Moved from `replicas` direct field to deploy-aware config
- **YAML generation**: Uses struct marshaling instead of Go templates - output format may differ
- **Project isolation**: Each stack now uses `-p name` for Docker Compose project isolation
- **New resource**: `dockercompose_project` for raw YAML workflows
- Existing v1 state must be destroyed and recreated

## Contributing

Issues and PRs welcome. If you hit a missing Compose field or a behavior that diverges from `docker compose`, open an issue with a minimal reproducer (one stack, the field, expected vs actual). For larger changes, open a discussion first so we can align on direction.

## License

[MIT](LICENSE) — use it however you want, attribution appreciated.

## Links

- **Terraform Registry**: <https://registry.terraform.io/providers/ritajhq/dockercompose/latest>
- **Issue tracker**: <https://github.com/ritajhq/terraform-provider-dockercompose/issues>
- **Releases / changelog**: <https://github.com/ritajhq/terraform-provider-dockercompose/releases>

If this provider saved you time, a star on GitHub helps others discover it.

