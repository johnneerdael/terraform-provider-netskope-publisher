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
