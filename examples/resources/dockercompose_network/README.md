# Example: `dockercompose_network` — network shared across stacks

Docker Compose prefixes every network declared inside a `dockercompose_stack`
with `<project>_`, so two stacks can't join "the same" network just by using
matching `network` blocks — `api-stack_shared_net` and
`worker-stack_shared_net` would end up as two separate networks.

`dockercompose_network` creates a Docker network directly (`docker network
create`), outside of any compose project, so its name is never prefixed.
Stacks that need to join it declare the network as `external = true` with
`external_name` pointing at the `dockercompose_network` resource's `name` —
this maps to Compose's top-level `networks.<key>.name` override, which pins
the literal network name instead of letting Compose derive one from the
project.

This example creates one shared network and two stacks (`api-stack`,
`worker-stack`) that both attach to it and can reach each other by service
name.

## Run

```bash
terraform init
terraform apply
```

```bash
docker network inspect shared_net
docker compose -p api-stack exec api ping worker
```

Tear it down (stacks first, so the network isn't removed while still in use):

```bash
terraform destroy
```

Terraform handles this ordering automatically via the `depends_on` on each
stack — `dockercompose_network.shared` is only destroyed after both stacks
that reference it are gone.
