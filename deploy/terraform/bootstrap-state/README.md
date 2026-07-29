# Bootstrap do state Terraform

Este diretório é executado uma vez com state local para criar o bucket S3
versionado e a chave KMS usados pelo Terraform principal.

```bash
terraform init
terraform fmt -check -recursive
terraform validate
terraform plan -out=tfplan
terraform apply tfplan
terraform output -raw backend_hcl
```

Copie o último output para `../aws/backend.hcl` e altere a `key` ao criar outro
ambiente. Guarde o state local deste bootstrap em local seguro; ele não deve
entrar no Git.

O locking usa o lockfile nativo do backend S3. Não é criado DynamoDB apenas
para locking.
