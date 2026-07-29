resource "aws_kms_key" "infrastructure" {
  description             = "Cumuru ${var.environment}: RDS, EBS, ECR, S3 e segredos"
  enable_key_rotation     = true
  deletion_window_in_days = 30

  tags = {
    Name = "${local.name}-infrastructure"
  }
}

resource "aws_kms_alias" "infrastructure" {
  name          = "alias/${local.name}-infrastructure"
  target_key_id = aws_kms_key.infrastructure.key_id
}
