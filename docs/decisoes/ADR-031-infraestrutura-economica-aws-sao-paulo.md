# ADR-031 — Infraestrutura econômica na AWS São Paulo

**Status:** aceito para automação de `staging` e piloto técnico
`PROTOTYPE_ONLY`.

## Contexto

O Cumuru precisa de uma infraestrutura reproduzível, barata e compatível com:

- PostgreSQL 17 com TLS autenticando o hostname;
- banco privado, backup contínuo e point-in-time recovery;
- imagens OCI reproduzíveis;
- segredos fora do repositório;
- criptografia em repouso;
- região próxima dos usuários da Bahia;
- caminho de evolução sem Kubernetes, Redis, Kafka ou microsserviços.

A menor mensalidade isolada não é o único critério. Hospedar PostgreSQL no
mesmo VPS reduz a fatura, mas concentra aplicação, banco e backup em uma única
falha e transfere patching, PITR e restore para a equipe. Provedores sem região
brasileira também adicionam latência e uma transferência internacional que
precisa de análise jurídica e contratual antes de dados pessoais.

Na consulta de preços de 29 de julho de 2026:

| Opção | Base mensal aproximada | Limitação determinante |
| --- | ---: | --- |
| Hetzner CX23 | US$ 6,49, sem IPv4/backup | sem região brasileira e sem PostgreSQL gerenciado |
| DigitalOcean 2 GiB + PostgreSQL + Spaces | US$ 32 | sem região brasileira |
| Lightsail 2 GiB + database + object storage | US$ 28 | documentação vigente lista PostgreSQL somente até 16 |
| AWS EC2 `t4g.small` + RDS `db.t4g.micro` | US$ 60–70 | maior custo, Single-AZ no baseline |

O preço AWS acima usa 730 horas e a tabela pública da região `sa-east-1`:

- EC2 `t4g.small`: US$ 0,0268/h, ou US$ 19,56/mês;
- EBS gp3 de 30 GiB: US$ 0,152/GiB-mês, ou US$ 4,56/mês;
- RDS PostgreSQL Single-AZ `db.t4g.micro`: US$ 0,034/h, ou
  US$ 24,82/mês;
- RDS gp3 de 20 GiB: US$ 0,219/GiB-mês, ou US$ 4,38/mês;
- um IPv4 público: US$ 0,005/h, ou US$ 3,65/mês;
- uma chave KMS gerenciada pelo cliente: US$ 1/mês;
- dois segredos no Secrets Manager: aproximadamente US$ 0,80/mês;
- Route 53, S3, ECR, requests e tráfego variam com o uso.

Impostos, domínio, transferência excedente, logs externos, OIDC e
observabilidade gerenciada não estão incluídos. O orçamento precisa ser
recalculado no AWS Pricing Calculator antes de cada contratação.

## Decisão

Adotar AWS `sa-east-1` como infraestrutura de referência econômica para
`staging` e futuro piloto autorizado:

- uma instância ARM `t4g.small` para Caddy, web, API e worker em contêineres;
- RDS PostgreSQL 17.10 privado, Single-AZ por padrão e Multi-AZ configurável;
- duas subnets privadas de banco em zonas distintas e uma subnet pública para
  a aplicação;
- nenhum NAT Gateway e nenhum load balancer no baseline;
- Elastic IP único, com SSH limitado por CIDR e somente HTTP/HTTPS públicos;
- ECR com tags imutáveis e scan no push para API/worker e web;
- S3 privado e versionado para SBOMs, manifests e artefatos;
- uma chave KMS por ambiente para EBS, RDS, ECR, S3 e Secrets Manager;
- Secrets Manager para o segredo mestre gerenciado pelo RDS e um segredo
  runtime materializado pelo Ansible;
- IAM de mínimo privilégio e IMDSv2 com hop limit 1;
- backups automáticos e PITR do RDS; snapshots finais e deletion protection
  configuráveis;
- alarmes essenciais e AWS Budget quando um e-mail de alerta for informado;
- DNS no Route 53 opcional; TLS público automatizado pelo Caddy.

Terraform cria somente recursos de nuvem. Ansible endurece o host, instala
Docker/Compose, inicializa os papéis PostgreSQL, materializa a configuração
secreta, executa migrations e sobe a release. Imagens são construídas e
publicadas separadamente, com metadados de build já exigidos pelo projeto.

O Compose de runtime não contém PostgreSQL: o banco continua gerenciado. O
Compose local permanece descartável e recebe um overlay opcional com
OpenTelemetry Collector, Prometheus, Tempo e Grafana para provar métricas e
traces sem tornar essa pilha um requisito da VM econômica.

## Limites fail-closed

- A automação não executa `terraform apply` nem deploy sem ação explícita.
- `staging` e `production` exigem OIDC real, issuer HTTPS e endpoint OTLP
  HTTPS; a automação não inventa o provedor institucional.
- A aplicação atual rejeita `PHASE3_ENABLED=true` e `PHASE4_ENABLED=true` fora
  de `local|test`. O deploy externo mantém ambas desabilitadas.
- KMS institucional, rotação, OIDC, restore real, observabilidade externa,
  WAF/CDN e operação Multi-AZ permanecem `UNVERIFIED`.
- O baseline Single-AZ não satisfaz sozinho uma topologia de produção com
  tolerância a falha de zona.
- Dados reais, piloto e release continuam bloqueados pelos gates jurídicos,
  de governança e segurança existentes.
- A chave KMS da infraestrutura protege segredos e discos, mas não substitui a
  integração futura de criptografia em envelope da aplicação.

## Evolução

Quando disponibilidade, carga ou autorização justificarem:

1. habilitar RDS Multi-AZ;
2. comprovar restore e rotação;
3. adicionar segunda instância, load balancer e health checks;
4. contratar CDN/WAF;
5. migrar telemetria para backend gerenciado;
6. adotar Savings Plans ou reservas somente após medir uso estável.

Kubernetes, banco autogerenciado e serviços distribuídos continuam fora do
escopo sem métricas e novo ADR.

## Fontes

- [AWS Price List — EC2 `sa-east-1`](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/sa-east-1/index.json)
- [AWS Price List — RDS `sa-east-1`](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonRDS/current/sa-east-1/index.json)
- [RDS PostgreSQL 17.10](https://docs.aws.amazon.com/AmazonRDS/latest/PostgreSQLReleaseNotes/postgresql-release-calendar.html)
- [Preço do IPv4 público](https://aws.amazon.com/vpc/pricing/)
- [Preço do KMS](https://aws.amazon.com/kms/pricing/)
- [Preço do Secrets Manager](https://aws.amazon.com/secrets-manager/pricing/)
- [Preços do Lightsail](https://aws.amazon.com/lightsail/pricing/)
- [Versões PostgreSQL no Lightsail](https://docs.aws.amazon.com/lightsail/latest/userguide/amazon-lightsail-choosing-a-database.html)
- [Preços da DigitalOcean](https://www.digitalocean.com/pricing)
- [Preços atuais da Hetzner](https://docs.hetzner.com/general/infrastructure-and-availability/price-adjustment/)
