terraform {
  required_providers {
    dockercompose = {
      source  = "ritajhq/dockercompose"
      version = "~> 1.1"
    }
  }
}

# Default configuration using local Docker instance
provider "dockercompose" {
  # (Optional) Defaults to the DOCKER_HOST environment variable if omitted.
  # host = "unix:///var/run/docker.sock"

  # (Optional) Provide a specific path to the docker executable.
  # docker_binary = "/usr/local/bin/docker"

  # (Optional) Override the base directory where docker-compose files and stack context will be saved.
  # project_directory = "/var/lib/terraform-docker-compose"

  # (Optional) Provider-level default for dockercompose_stack's `watch` attribute.
  # Individual stacks can still override this by setting `watch` explicitly, so a
  # single stack config can be reused across dev (watch = true) and other
  # environments (watch = false) just by toggling this — or a per-stack override.
  # watch = false

  # (Optional) Provider-level default for dockercompose_stack's `active_profiles`
  # attribute. Individual stacks can override this by setting `active_profiles`
  # explicitly (including [] to force no profiles active).
  # active_profiles = []
}
