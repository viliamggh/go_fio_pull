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

# Reference core infrastructure from fin_az_core Terraform state
data "terraform_remote_state" "core_infra" {
  backend = "azurerm"
  config = {
    resource_group_name  = var.core_infra_rg
    storage_account_name = var.core_infra_sa
    container_name       = var.core_infra_container
    key                  = var.core_infra_key
  }
}

# Data source for go_fio_pull's OWN resource group (for app resources)
data "azurerm_resource_group" "app_rg" {
  name = var.rg_name
}

# Local variables for go_fio_pull's own resources
locals {
  # App resource group (where Container App lives)
  app_rg_name     = data.azurerm_resource_group.app_rg.name
  app_rg_location = data.azurerm_resource_group.app_rg.location
  
  # Core infrastructure references (from fin_az_core remote state)
  storage_account_name     = data.terraform_remote_state.core_infra.outputs.storage_account_name
  storage_account_url      = data.terraform_remote_state.core_infra.outputs.storage_account_url
  storage_account_id       = data.terraform_remote_state.core_infra.outputs.storage_account_id
  key_vault_url            = data.terraform_remote_state.core_infra.outputs.key_vault_url
  key_vault_name           = data.terraform_remote_state.core_infra.outputs.key_vault_name
  key_vault_id             = data.terraform_remote_state.core_infra.outputs.key_vault_id
  acr_login_server         = data.terraform_remote_state.core_infra.outputs.container_registry_login_server
  acr_id                   = data.terraform_remote_state.core_infra.outputs.container_registry_id
  storage_container_name   = data.terraform_remote_state.core_infra.outputs.storage_container_raw_name
}

resource "azurerm_container_app_environment" "c_app_env" {
  name                = "${var.project_name_no_dash}cae"
  location            = local.app_rg_location
  resource_group_name = local.app_rg_name
  # log_analytics_workspace_id = azurerm_log_analytics_workspace.example.id
}

resource "azurerm_user_assigned_identity" "c_app_identity" {
  location            = local.app_rg_location
  name                = "${var.project_name_no_dash}acaid"
  resource_group_name = local.app_rg_name
}

resource "null_resource" "always_run" {
  triggers = {
    timestamp = "${timestamp()}"
  }
}

resource "azurerm_role_assignment" "c_app_acrpull" {
  scope                = local.acr_id
  role_definition_name = "AcrPull"
  principal_id         = azurerm_user_assigned_identity.c_app_identity.principal_id
}

resource "azurerm_role_assignment" "c_app_storage_access" {
  scope                = local.storage_account_id
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = azurerm_user_assigned_identity.c_app_identity.principal_id

  depends_on = [azurerm_container_app.bank_pull]
}

resource "azurerm_role_assignment" "c_app_keyvault_access" {
  scope                = local.key_vault_id
  role_definition_name = "Key Vault Secrets User"
  principal_id         = azurerm_user_assigned_identity.c_app_identity.principal_id

  depends_on = [azurerm_container_app.bank_pull]
}




