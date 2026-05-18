# terraform-provider-netskope-publisher

> **Unofficial.** This provider is a small, focused implementation that exposes
> only the Netskope NPA endpoints needed to manage **publishers** and their
> **registration tokens**. It is *not* affiliated with Netskope.
>
> For the full Netskope API surface, prefer the official
> [`netskopeoss/netskope`](https://registry.terraform.io/providers/netskopeoss/netskope)
> provider.

Targets the
[`johnneerdael/terraform-netskope-publisher`](https://github.com/johnneerdael/terraform-netskope-publisher)
module — it lets the module manage publisher records and tokens as
first-class Terraform resources instead of via `http` data sources.

## Install

```hcl
terraform {
  required_providers {
    npa = {
      source  = "johnneerdael/netskope-publisher"
      version = "~> 0.1"
    }
  }
}

provider "npa" {
  tenant_url = "https://tenant.goskope.com"
  api_token  = "..." # or set NETSKOPE_API_TOKEN env var
}
```

## Resources

### `npa_publisher`

Full CRUD on a publisher record.

```hcl
resource "npa_publisher" "primary" {
  name = "pub-eu-1"
}
```

**Arguments:** `name` (required).
**Attributes:** `id`, `publisher_id`, `common_name`, `registered`, `status`.

Idempotent create — if a publisher with the same name already exists in the
tenant, the resource adopts it instead of erroring.

### `npa_publisher_token`

Issues a one-shot registration token for a publisher. Sensitive.

```hcl
resource "npa_publisher_token" "primary" {
  publisher_id = npa_publisher.primary.id
}
```

**Arguments:** `publisher_id` (required, forces replacement).
**Attributes:** `id`, `token` (sensitive).

Re-issue a token by tainting the resource:

```bash
terraform taint npa_publisher_token.primary
terraform apply
```

The provider has no DELETE for tokens — they expire server-side or are consumed
on first boot by `/home/ubuntu/npa_publisher_wizard -token <token>`.

## Provider configuration

| Name | Type | Required | Description |
|---|---|---|---|
| `tenant_url` | string | yes | Netskope tenant URL, must start with `https://`. Falls back to `NETSKOPE_TENANT_URL`. |
| `api_token` | string (sensitive) | yes | NPA API token with publisher read/write scope. Falls back to `NETSKOPE_API_TOKEN`. |

## Building from source

```bash
go build -o terraform-provider-netskope-publisher .
```

Then drop the binary into your Terraform plugin directory to test locally:

```bash
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/johnneerdael/netskope-publisher/0.1.0/darwin_arm64
cp terraform-provider-netskope-publisher ~/.terraform.d/plugins/registry.terraform.io/johnneerdael/netskope-publisher/0.1.0/darwin_arm64/
```

(Adjust `darwin_arm64` for your OS/arch.)

## License

[Mozilla Public License 2.0](LICENSE)
