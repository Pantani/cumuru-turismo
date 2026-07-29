#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TERRAFORM_DIR="${TERRAFORM_DIR:-${ROOT_DIR}/deploy/terraform/aws}"
ANSIBLE_DIR="${ROOT_DIR}/deploy/ansible"
INVENTORY_FILE="${ANSIBLE_DIR}/inventory/hosts.yml"
VARS_FILE="${ANSIBLE_DIR}/group_vars/all/generated.yml"

command -v terraform >/dev/null 2>&1 ||
  {
    echo "terraform is required" >&2
    exit 2
  }

output() {
  terraform -chdir="${TERRAFORM_DIR}" output -raw "$1"
}

public_ip="$(output application_public_ip)"
environment="$(output environment)"
region="$(output aws_region)"
domain="$(output domain_name)"
rds_address="$(output rds_address)"
master_secret="$(output rds_master_secret_arn)"
runtime_secret="$(output runtime_secret_arn)"
api_repository="$(output api_repository_url)"
web_repository="$(output web_repository_url)"
artifact_bucket="$(output artifact_bucket)"

mkdir -p "$(dirname "${INVENTORY_FILE}")" "$(dirname "${VARS_FILE}")"

{
  printf '%s\n' '---'
  printf '%s\n' 'all:'
  printf '%s\n' '  children:'
  printf '%s\n' '    cumuru:'
  printf '%s\n' '      hosts:'
  printf '        %s:\n' "${environment}"
  printf '          ansible_host: %s\n' "${public_ip}"
  printf '%s\n' '          ansible_user: ubuntu'
} >"${INVENTORY_FILE}"

{
  printf '%s\n' '---'
  printf 'cumuru_environment: %s\n' "${environment}"
  printf 'cumuru_aws_region: %s\n' "${region}"
  printf 'cumuru_domain: %s\n' "${domain}"
  printf 'cumuru_rds_address: %s\n' "${rds_address}"
  printf 'cumuru_rds_master_secret_arn: %s\n' "${master_secret}"
  printf 'cumuru_runtime_secret_arn: %s\n' "${runtime_secret}"
  printf 'cumuru_api_repository_url: %s\n' "${api_repository}"
  printf 'cumuru_web_repository_url: %s\n' "${web_repository}"
  printf 'cumuru_artifact_bucket: %s\n' "${artifact_bucket}"
} >"${VARS_FILE}"

chmod 0600 "${INVENTORY_FILE}" "${VARS_FILE}"
echo "Ansible inventory rendered from Terraform outputs"
