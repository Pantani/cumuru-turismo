variable "aws_region" {
  type    = string
  default = "sa-east-1"

  validation {
    condition     = var.aws_region == "sa-east-1"
    error_message = "O backend deve permanecer em sa-east-1."
  }
}

variable "project_name" {
  type    = string
  default = "cumuru"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,20}$", var.project_name))
    error_message = "project_name inválido."
  }
}
