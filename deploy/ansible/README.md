# Ansible

Configura a única VM do baseline econômico e publica uma release já existente
no ECR. O playbook não cria recursos AWS; isso pertence ao Terraform.

## Pré-requisitos

- `ansible-core` 2.17 ou superior;
- collection `community.docker` 5.2.1;
- Terraform aplicado e DNS apontando para o Elastic IP;
- imagens ARM publicadas com tag e digest imutável;
- issuer/audience OIDC institucionais;
- endpoint OTLP HTTPS;
- acesso SSH pelo `admin_cidr`.

```bash
ansible-galaxy collection install -r requirements.yml
cp inventory/hosts.yml.example inventory/hosts.yml
cp group_vars/all/main.yml.example group_vars/all/main.yml
```

O script `../scripts/render-ansible-inventory.sh` gera os outputs não secretos
do Terraform. Complete `group_vars/all/main.yml` com OIDC, OTLP, e-mail ACME e
tag da release, além de `cumuru_api_image_digest` e
`cumuru_web_image_digest` exatamente como retornados por
`deploy/scripts/publish-images.sh`.

## Validação e execução

```bash
ansible-playbook playbooks/site.yml --syntax-check
ansible-playbook playbooks/site.yml --check --diff
ansible-playbook playbooks/site.yml
```

O primeiro run:

1. instala Docker Compose v2 e ferramentas do host;
2. busca ou cria o segredo runtime no Secrets Manager;
3. busca o segredo mestre administrado pelo RDS;
4. cria os papéis PostgreSQL mínimos;
5. baixa o bundle oficial de CAs do RDS;
6. baixa imagens imutáveis;
7. aplica migrations;
8. sobe Caddy, web, API e worker;
9. valida health e readiness por HTTPS.

Senhas e keyrings não aparecem no repositório, output normal ou Terraform
state. O arquivo runtime no host é `0640`, e valores temporários em `/run` são
removidos.

## Limites

- O check mode não consegue provar operações que dependem de uma VM recém
  criada, Secrets Manager ou Docker.
- Caddy descarta logs de borda para impedir vazamento de capabilities no path.
- O host ainda pode acessar os dois segredos para permitir deploy e
  provisionamento; containers não alcançam IMDS porque o Terraform usa hop
  limit 1.
- A automação não prova restore, OIDC, OTLP ou operação real.
- Fases 3 e 4 ficam desabilitadas em `staging|production` porque a aplicação
  atual falha fechado nesses ambientes.
