terraform {
  backend "azurerm" {
  }
  required_providers {
    azurerm = {
      source = "hashicorp/azurerm"
    }
    azuread = {
      source = "hashicorp/azuread"
    }
  }
}

provider "azurerm" {
  features {}
}

data "azurerm_client_config" "current" {}
data "azurerm_resource_group" "rg" {
  name = var.rg_name
}
data "azurerm_container_registry" "acr" {
  name                = var.acr_name
  resource_group_name = data.azurerm_resource_group.rg.name
}
# data "azurerm_user_assigned_identity" "cicd_principal" {
#   name                = "name_of_user_assigned_identity"
#   resource_group_name = "name_of_resource_group"
# }

resource "azurerm_key_vault" "kv" {
  name                        = "${var.project_name_no_dash}kv"
  location                    = data.azurerm_resource_group.rg.location
  resource_group_name         = data.azurerm_resource_group.rg.name
  enabled_for_disk_encryption = true
  tenant_id                   = data.azurerm_client_config.current.tenant_id
  soft_delete_retention_days  = 7
  purge_protection_enabled    = false
  enable_rbac_authorization   = true

  sku_name = "standard"
}

# resource "azurerm_role_assignment" "kv_rbac_vg" {
#   scope                = azurerm_key_vault.kv.id
#   role_definition_name = "Key Vault Secrets Officer"
#   principal_id         = "032fd23a-f163-4727-ba07-4260f5daa653"
# }

resource "azurerm_storage_account" "sa" {
  name                            = "${var.project_name_no_dash}sa"
  location                        = data.azurerm_resource_group.rg.location
  resource_group_name             = data.azurerm_resource_group.rg.name
  account_tier                    = "Standard"
  account_replication_type        = "LRS"
  public_network_access_enabled   = true
  default_to_oauth_authentication = true

  # network_rules {
  #   // Deny all by default
  #   default_action = "Deny"

  #   // Let Azure services bypass the network restrictions
  #   // (Logging, Metrics, AzureServices can be added if desired)
  #   bypass = [
  #     "AzureServices"
  #   ]

  #   # // Only allow public access from this IP address
  #   # ip_rules = [
  #   #   "109.81.90.2"
  #   # ]
  # }
}

resource "azurerm_storage_container" "data_cont" {
  name               = "raw"
  storage_account_id = azurerm_storage_account.sa.id
}

resource "azurerm_container_app_environment" "c_app_env" {
  name                = "${var.project_name_no_dash}cae"
  location            = data.azurerm_resource_group.rg.location
  resource_group_name = data.azurerm_resource_group.rg.name
  # log_analytics_workspace_id = azurerm_log_analytics_workspace.example.id
}

resource "azurerm_user_assigned_identity" "c_app_identity" {
  location            = data.azurerm_resource_group.rg.location
  name                = "${var.project_name_no_dash}acaid"
  resource_group_name = data.azurerm_resource_group.rg.name
}

resource "null_resource" "always_run" {
  triggers = {
    timestamp = "${timestamp()}"
  }
}

resource "azurerm_role_assignment" "c_app_acrpull" {
  scope                = data.azurerm_container_registry.acr.id
  role_definition_name = "AcrPull"
  principal_id         = azurerm_user_assigned_identity.c_app_identity.principal_id
  # principal_id = azurerm_container_app.example.identity[0].principal_id
}

resource "azurerm_role_assignment" "c_app_storage_access" {
  scope                = azurerm_storage_account.sa.id
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = azurerm_user_assigned_identity.c_app_identity.principal_id

  depends_on = [azurerm_container_app.bank_pull]
}



