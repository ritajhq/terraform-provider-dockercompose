terraform {
  required_providers {
    dockercompose = {
      source  = "xRizur/dockercompose"
      version = "~> 1.1"
    }
  }
}

resource "dockercompose_project" "legacy_app" {
  name = "backend-services"

  # Option 1: Point to an existing file on the disk
  compose_file = "/var/app/backend-services/docker-compose.yml"

  # Make sure we clean named volumes when tearing down the project via terraform destroy
  remove_volumes_on_destroy = true
}

resource "dockercompose_project" "yaml_app" {
  name = "inline-services"

  # Option 2: Provide the YAML directly within Terraform, very useful with templatefile()
  compose_yaml = <<-EOT
    version: '3'
    services:
      web:
        image: nginx:latest
        ports:
          - "80:80"
  EOT

  remove_volumes_on_destroy = true
}

# Toggle this to enable "docker compose up --watch" for local dev, leave it
# false everywhere else (staging/prod/CI). watch is intended for local dev
# only: it starts a long-running, detached watcher process attached to this
# project and is not meant for CI/prod applies. develop.watch entries must
# already be present in the underlying compose YAML/file for this to have
# any effect.
variable "enable_watch" {
  type    = bool
  default = false
}

resource "dockercompose_project" "webapp" {
  name  = "webapp-dev"
  watch = var.enable_watch

  compose_yaml = <<-EOT
    services:
      web:
        image: myapp:dev
        ports:
          - "3000:3000"
        volumes:
          - ./src:/app/src
        develop:
          watch:
            - path: ./src
              action: sync
              target: /app/src
              ignore:
                - node_modules/
            - path: ./package.json
              action: rebuild
  EOT
}
