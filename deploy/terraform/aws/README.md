# Terraform AWS

Cria o baseline econômico documentado no ADR-031. Nenhum comando deste
diretório executa deploy automaticamente.

## Pré-requisitos

- Terraform entre 1.11 e 1.x;
- credenciais AWS temporárias ou SSO, nunca access keys no repositório;
- região `sa-east-1` habilitada;
- uma chave pública SSH;
- domínio sob controle do operador;
- backend de state criado por `../bootstrap-state`.

## Inicialização

```bash
cp backend.hcl.example backend.hcl
cp terraform.tfvars.example terraform.tfvars
# Edite os dois arquivos sem inserir credenciais AWS.

terraform init -backend-config=backend.hcl
terraform fmt -check -recursive
terraform validate
terraform plan -out=tfplan
terraform show tfplan
```

Somente depois de revisão humana:

```bash
terraform apply tfplan
```

`terraform.tfvars`, `backend.hcl`, `tfplan` e todo state são ignorados pelo
Git. O state continua sensível mesmo sem senha explícita.

## Depois do apply

```bash
../../scripts/render-ansible-inventory.sh
../../scripts/publish-images.sh
ansible-playbook ../../ansible/playbooks/site.yml
```

Consulte os READMEs dos scripts e do Ansible para os parâmetros obrigatórios.

## Custos

O output `estimated_core_monthly_usd` é somente uma faixa histórica. Use AWS
Pricing Calculator e revise o `terraform plan` antes da contratação. Multi-AZ,
logs, tráfego, OIDC, observabilidade externa e impostos aumentam a fatura.

## Destruição

Não use `terraform destroy` em banco com dados. Com deletion protection
habilitada, a remoção falha fechado. O procedimento correto exige:

1. aprovação explícita;
2. restore testado;
3. snapshot final nomeado;
4. retenção legal revisada;
5. alteração consciente de `rds_deletion_protection`.
