output "bucket" {
  value = aws_s3_bucket.state.id
}

output "kms_key_arn" {
  value = aws_kms_key.state.arn
}

output "backend_hcl" {
  description = "Copie e ajuste a key por ambiente."
  value       = <<-EOT
    bucket       = "${aws_s3_bucket.state.id}"
    key          = "cumuru/staging/terraform.tfstate"
    region       = "${var.aws_region}"
    encrypt      = true
    use_lockfile = true
    kms_key_id   = "${aws_kms_key.state.arn}"
  EOT
}
