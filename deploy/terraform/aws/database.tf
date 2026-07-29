resource "aws_db_subnet_group" "postgres" {
  name       = "${local.name}-postgres"
  subnet_ids = [for subnet in aws_subnet.database : subnet.id]

  tags = {
    Name = "${local.name}-postgres"
  }
}

resource "aws_db_instance" "postgres" {
  identifier = "${local.name}-postgres"

  engine         = "postgres"
  engine_version = var.db_engine_version
  instance_class = var.db_instance_class

  db_name  = "cumuru"
  username = "cumuru_admin"
  port     = 5432

  manage_master_user_password   = true
  master_user_secret_kms_key_id = aws_kms_key.infrastructure.arn

  allocated_storage     = var.db_allocated_storage_gib
  max_allocated_storage = var.db_max_allocated_storage_gib
  storage_type          = "gp3"
  storage_encrypted     = true
  kms_key_id            = aws_kms_key.infrastructure.arn

  db_subnet_group_name   = aws_db_subnet_group.postgres.name
  vpc_security_group_ids = [aws_security_group.database.id]
  publicly_accessible    = false
  multi_az               = var.rds_multi_az

  backup_retention_period = 7
  backup_window           = "05:00-05:30"
  maintenance_window      = "sun:06:00-sun:07:00"

  auto_minor_version_upgrade  = true
  allow_major_version_upgrade = false
  apply_immediately           = false
  copy_tags_to_snapshot       = true
  deletion_protection         = var.rds_deletion_protection
  delete_automated_backups    = false
  enabled_cloudwatch_logs_exports = [
    "postgresql",
    "upgrade",
  ]

  skip_final_snapshot       = !var.rds_final_snapshot
  final_snapshot_identifier = var.rds_final_snapshot ? "${local.name}-postgres-final" : null

  tags = {
    Name              = "${local.name}-postgres"
    BackupRequirement = "pitr-and-restore-test"
  }
}
