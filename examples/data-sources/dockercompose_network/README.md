# Example: `dockercompose_network` data source — reference a network you don't manage

Sometimes the network you want to join already exists and isn't (and
shouldn't be) managed by this Terraform config at all — created by another
team, a bootstrap script, or `docker network create` run by hand. This
example looks up a network named `edge` by name and uses it to join a stack
to it, without Terraform ever creating or destroying that network.

If `edge` doesn't exist, `terraform plan`/`apply` fails immediately with a
clear error instead of silently doing nothing useful or creating an
unrelated project-prefixed network.

## Run

```bash
docker network create edge   # simulate a network created outside Terraform
terraform init
terraform apply
```

```bash
docker network inspect edge
docker compose -p api-stack exec api ip addr
```

## Related

- [`resources/dockercompose_network/`](../../resources/dockercompose_network/) — create and own a shared network's lifecycle with Terraform instead of just referencing one
