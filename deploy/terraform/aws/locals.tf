locals {
  name = "${var.project_name}-${var.environment}"

  availability_zones = slice(data.aws_availability_zones.available.names, 0, 2)

  artifact_bucket_name = "${local.name}-artifacts-${data.aws_caller_identity.current.account_id}"
  runtime_secret_name  = "${var.project_name}/${var.environment}/runtime"

  alarm_actions = var.alert_email == null ? [] : [aws_sns_topic.alerts[0].arn]
}
