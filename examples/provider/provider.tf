terraform {
  required_providers {
    redfish = { source = "simonzrln/redfish" }
  }
}

variable "bmc_password" {
  type      = string
  sensitive = true
}

# One entry per BMC; resources pick it with redfish_server { redfish_alias = "bmc" }.
provider "redfish" {
  redfish_servers = {
    bmc = {
      user         = "admin"
      password     = var.bmc_password
      endpoint     = "https://192.0.2.10"
      ssl_insecure = true # AMI ships a self-signed certificate
    }
  }
}
