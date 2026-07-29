variable "project_name" {
  description = "Prefixo curto usado nos recursos."
  type        = string
  default     = "cumuru"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,20}$", var.project_name))
    error_message = "project_name deve usar letras minúsculas, números e hífen."
  }
}

variable "environment" {
  description = "Ambiente externo. O protótipo local não usa este Terraform."
  type        = string
  default     = "staging"

  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "environment deve ser staging ou production."
  }
}

variable "aws_region" {
  description = "Região AWS aprovada para o baseline brasileiro."
  type        = string
  default     = "sa-east-1"

  validation {
    condition     = var.aws_region == "sa-east-1"
    error_message = "Este baseline foi analisado e orçado somente para sa-east-1."
  }
}

variable "ssh_public_key" {
  description = "Chave pública SSH usada pelo Ansible."
  type        = string

  validation {
    condition     = can(regex("^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp)", trimspace(var.ssh_public_key)))
    error_message = "ssh_public_key deve ser uma chave pública OpenSSH."
  }
}

variable "admin_cidr" {
  description = "Único CIDR IPv4 autorizado a acessar SSH."
  type        = string

  validation {
    condition     = can(cidrhost(var.admin_cidr, 0)) && var.admin_cidr != "0.0.0.0/0"
    error_message = "admin_cidr deve ser um CIDR IPv4 restrito; 0.0.0.0/0 é proibido."
  }
}

variable "domain_name" {
  description = "FQDN público do ambiente, sem protocolo."
  type        = string

  validation {
    condition = (
      can(regex("^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$", var.domain_name))
      && !endswith(var.domain_name, ".")
    )
    error_message = "domain_name deve ser um FQDN minúsculo válido."
  }
}

variable "route53_zone_id" {
  description = "Hosted zone existente. Null mantém o DNS fora do Terraform."
  type        = string
  default     = null
  nullable    = true
}

variable "instance_type" {
  description = "Instância ARM para API, worker, web e Caddy."
  type        = string
  default     = "t4g.small"

  validation {
    condition     = contains(["t4g.small", "t4g.medium", "t4g.large"], var.instance_type)
    error_message = "Use uma instância ARM t4g.small, t4g.medium ou t4g.large."
  }
}

variable "root_volume_gib" {
  description = "Tamanho do volume gp3 da aplicação."
  type        = number
  default     = 30

  validation {
    condition     = var.root_volume_gib >= 20 && var.root_volume_gib <= 200
    error_message = "root_volume_gib deve estar entre 20 e 200 GiB."
  }
}

variable "db_instance_class" {
  description = "Classe do RDS PostgreSQL."
  type        = string
  default     = "db.t4g.micro"

  validation {
    condition     = can(regex("^db\\.t4g\\.(micro|small|medium|large)$", var.db_instance_class))
    error_message = "Use uma classe ARM db.t4g entre micro e large."
  }
}

variable "db_engine_version" {
  description = "Versão PostgreSQL validada pelo projeto."
  type        = string
  default     = "17.10"

  validation {
    condition     = can(regex("^17\\.[0-9]+$", var.db_engine_version))
    error_message = "A versão do RDS deve permanecer na linha PostgreSQL 17."
  }
}

variable "db_allocated_storage_gib" {
  description = "Storage gp3 inicial do RDS."
  type        = number
  default     = 20

  validation {
    condition     = var.db_allocated_storage_gib >= 20 && var.db_allocated_storage_gib <= 1000
    error_message = "db_allocated_storage_gib deve estar entre 20 e 1000 GiB."
  }
}

variable "db_max_allocated_storage_gib" {
  description = "Limite de autoscaling do storage do RDS."
  type        = number
  default     = 100

  validation {
    condition     = var.db_max_allocated_storage_gib >= var.db_allocated_storage_gib
    error_message = "db_max_allocated_storage_gib não pode ser menor que o storage inicial."
  }
}

variable "rds_multi_az" {
  description = "Habilita standby em outra zona. Aumenta significativamente o custo."
  type        = bool
  default     = false
}

variable "rds_deletion_protection" {
  description = "Impede exclusão acidental do banco."
  type        = bool
  default     = true
}

variable "rds_final_snapshot" {
  description = "Exige snapshot final ao destruir o banco."
  type        = bool
  default     = true
}

variable "disable_instance_termination" {
  description = "Protege a VM contra terminação pela API."
  type        = bool
  default     = false
}

variable "artifact_retention_days" {
  description = "Retenção das versões antigas de artefatos no S3."
  type        = number
  default     = 365

  validation {
    condition     = var.artifact_retention_days >= 90
    error_message = "artifact_retention_days deve ser pelo menos 90."
  }
}

variable "alert_email" {
  description = "E-mail opcional para Budget e alarmes. A assinatura SNS exige confirmação."
  type        = string
  default     = null
  nullable    = true

  validation {
    condition     = var.alert_email == null || can(regex("^[^@[:space:]]+@[^@[:space:]]+\\.[^@[:space:]]+$", var.alert_email))
    error_message = "alert_email deve ser null ou um e-mail válido."
  }
}

variable "monthly_budget_usd" {
  description = "Limite mensal do AWS Budget."
  type        = number
  default     = 100

  validation {
    condition     = var.monthly_budget_usd >= 20
    error_message = "monthly_budget_usd deve ser pelo menos US$ 20."
  }
}
