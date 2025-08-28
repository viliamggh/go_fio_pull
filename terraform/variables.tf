variable "acr_name" {
  type = string
}

variable "rg_name" {
  type = string
}

variable "project_name_no_dash" {
  type = string
}

variable "image_name" {
  type = string
  default = "bank_pull"
}

variable "environment" {
  type = string
  default = "main"
}