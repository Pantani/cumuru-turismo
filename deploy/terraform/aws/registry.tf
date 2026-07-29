resource "aws_ecr_repository" "runtime" {
  for_each = toset(["api", "web"])

  name                 = "${local.name}-${each.key}"
  image_tag_mutability = "IMMUTABLE"

  encryption_configuration {
    encryption_type = "KMS"
    kms_key         = aws_kms_key.infrastructure.arn
  }

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = {
    Component = each.key
  }
}

resource "aws_ecr_lifecycle_policy" "runtime" {
  for_each   = aws_ecr_repository.runtime
  repository = each.value.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Manter as 20 releases mais recentes"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 20
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}
