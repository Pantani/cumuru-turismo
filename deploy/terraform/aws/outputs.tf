output "application_public_ip" {
  description = "IPv4 para DNS e inventory Ansible."
  value       = aws_eip.application.public_ip
}

output "application_instance_id" {
  description = "ID da instância para operação via SSM."
  value       = aws_instance.application.id
}

output "domain_name" {
  value = var.domain_name
}

output "environment" {
  value = var.environment
}

output "aws_region" {
  value = var.aws_region
}

output "rds_endpoint" {
  description = "Endpoint privado com porta."
  value       = aws_db_instance.postgres.endpoint
}

output "rds_address" {
  description = "Hostname usado pelo sslmode=verify-full."
  value       = aws_db_instance.postgres.address
}

output "rds_master_secret_arn" {
  description = "Segredo administrado pelo RDS; o valor não entra no state."
  value       = aws_db_instance.postgres.master_user_secret[0].secret_arn
}

output "runtime_secret_arn" {
  description = "Segredo inicializado pelo Ansible."
  value       = aws_secretsmanager_secret.runtime.arn
}

output "artifact_bucket" {
  value = aws_s3_bucket.artifacts.id
}

output "api_repository_url" {
  value = aws_ecr_repository.runtime["api"].repository_url
}

output "web_repository_url" {
  value = aws_ecr_repository.runtime["web"].repository_url
}

output "kms_key_arn" {
  value = aws_kms_key.infrastructure.arn
}

output "estimated_core_monthly_usd" {
  description = "Snapshot informativo de 2026-07-29; não é cotação."
  value       = var.rds_multi_az ? "aproximadamente 90-110 mais uso e impostos" : "aproximadamente 60-70 mais uso e impostos"
}
