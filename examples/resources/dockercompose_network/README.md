# Example: `dockercompose_network` — network shared across stacks

Docker Compose prefixes every network declared inside a `dockercompose_stack`
with `<project>_`, so two stacks can't join "the same" network just by using
matching `network` blocks — `platform-stack_edge_net` and
`api-stack_edge_net` would end up as two separate networks.

`dockercompose_network` creates a Docker network directly (`docker network
create`), outside of any compose project, so its name is never prefixed.
Stacks that need to join it point `external_name` at the
`dockercompose_network` resource's `name` (this implies `external = true` —
no need to set that separately). That maps to Compose's top-level
`networks.<key>.name` override, which pins the literal network name instead
of letting Compose derive one from the project.

## What this example builds

Three resources sharing one network (`edge_net`):

- **`dockercompose_network.edge`** — the shared network itself, with a fixed
  subnet/gateway.
- **`dockercompose_stack.platform`** — owns the backing services: Postgres
  (with a healthcheck and a named volume) and Redis.
- **`dockercompose_stack.api`** — a separate stack (separate compose project,
  separate `terraform.tfstate` node) with an `api` service and a `worker`
  service. Both reach Postgres and Redis purely by service name
  (`postgres:5432`, `redis:6379`) because they're on the same Docker network
  — no service discovery, no hardcoded container IPs.

This is the realistic shape of the problem `dockercompose_network` solves:
splitting an app into independently-deployable stacks (so the API can be
redeployed without touching the database) while still letting them talk to
each other directly.

## Run

```bash
terraform init
terraform apply
```

```bash
docker network inspect edge_net
docker compose -p api-stack exec api sh -c 'nc -zv postgres 5432 && nc -zv redis 6379'
curl localhost:8080
```

Tear it down (stacks first, so the network isn't removed while still in use):

```bash
terraform destroy
```

Terraform handles this ordering automatically via `depends_on` — the API
stack depends on both the network and the platform stack, and the platform
stack depends on the network, so `dockercompose_network.edge` is only
destroyed last, after every stack that references it is gone.

## Related

- [`data-sources/dockercompose_network/`](../../data-sources/dockercompose_network/) — join a network that already exists and isn't managed by this Terraform config at all
