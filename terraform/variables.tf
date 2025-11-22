variable "rg_name" {
  type        = string
  description = "Resource group name for go_fio_pull microservice resources (Container App, Identity)"
}

variable "project_name_no_dash" {
  type        = string
  description = "Project name without dashes for resource naming"
}

variable "image_name" {
  type        = string
  default     = "bank_pull"
  description = "Container image name"
}

variable "environment" {
  type        = string
  default     = "main"
  description = "Environment tag for the image"
}

# Core infrastructure remote state configuration (passed from .infra-refs.yaml at runtime)
variable "core_infra_rg" {
  type        = string
  description = "Resource group where fin_az_core terraform state is stored"
}

variable "core_infra_sa" {
  type        = string
  description = "Storage account where fin_az_core terraform state is stored"
}

variable "core_infra_container" {
  type        = string
  description = "Container where fin_az_core terraform state is stored"
}

variable "core_infra_key" {
  type        = string
  description = "State file key for fin_az_core terraform state"
}
