# Fundação de execução local

Os arquivos deste diretório existem somente para o protótipo local com dados
fictícios. As senhas presentes no bootstrap do PostgreSQL são públicas,
deliberadamente locais e não podem ser reutilizadas em outro ambiente.

- `postgres/init/`: cria roles de grupo e logins do Compose antes das migrations;
- `scripts/test-migrations.sh`: usa um projeto Compose temporário e volume
  descartável, exige somente o par `000001_initial_schema` e testa
  `zero → 1 → zero → 1`, grants e tenancy;
- `scripts/test-local-restore.sh`: usa PostgreSQL real em projeto descartável,
  aplica a baseline, grava somente canários fictícios, executa dump/restore em
  base isolada e compara schema, ownership, grants e integridade antes do
  cleanup;
- `scripts/test-phase2-integration.sh`: usa projeto, volume e porta efêmeros,
  aplica migrations e fixtures com `cumuru_migration` e executa o teste tagged
  com conexão runtime `cumuru_app`; cleanup falha se deixar recurso residual;
- `scripts/test-phase3-integration.sh`: aplica a baseline consolidada em
  PostgreSQL efêmero e executa a suíte tagged com o papel runtime;
- `scripts/test-phase3-proxy.sh`: preserva o canário de proxy e verifica o
  contrato do header `Survey-Capability` sem criar log ou rewrite do segredo;
- `scripts/test-phase4-integration.sh`: aplica a baseline consolidada em
  PostgreSQL efêmero e prova publicação protegida, views públicas e negação das
  tabelas-base e do texto livre criptografado;
- `scripts/test-phase4-proxy.sh`: valida autenticação vazia e seletores
  fechados; resolve recursivamente os schemas públicos para exigir objetos
  fechados e rejeitar chaves proibidas; prova headers `200`/`304`, cache, ETag
  forte e `no-store` nos erros;
- `scripts/test-phase4-full-stack.sh`: valida o overlay Compose e a separação
  entre o DSN transacional e o login PostgreSQL público, usando o bootstrap Go
  como fonte canônica da fixture;
- `scripts/test-local-demo.sh`: prova banco fresh, repetição, preservação,
  concorrência sob advisory lock de sessão, rollover de datas e colisão
  fail-closed sem sobrescrever linhas existentes; usa
  `compose.local-test.yaml` e subnet própria para não disputar o pool do
  full-stack executado logo depois;
- `scripts/test-local-demo-e2e.sh`: sobe stack efêmera em porta e allowlist CORS
  exatas, executa `e2e/local-demo.spec.ts` no Chromium e confirma cleanup de
  containers, rede e volume; o E2E exige Service Worker registrado e rejeita
  API ou capability no Cache API;
- `scripts/test-proxy-hardening.sh`: prova headers fail-closed e ausência de
  capability/dados de erro nos logs de Nginx e Vite;
- `scripts/test-phase2-full-stack.sh`: constrói e testa API, worker, web e banco
  em projeto efêmero, incluindo canário com API indisponível, restauração e
  cleanup verificável;
- `scripts/check-generated.sh`: regenera cliente OpenAPI e código `sqlc` em
  diretório temporário e compara byte a byte;
- `scripts/test-complexity.sh`: prova o limiar 10 em fixtures temporárias,
  incluindo `_test.go`, e mede Go e todo código web próprio com máximo 9;
- `scripts/smoke.sh`: testa API, web, proxy e uma rota autenticada da Fase 2
  com o fake OIDC local depois de `make up`; ao invocar diretamente o perfil
  `local-demo`, exige `LOCAL_FAKE_OIDC_TOKEN` explícito.

O Nginx serve deep links da SPA e é a única entrada host para `/api/v1/**`; a
API não publica porta. O proxy mantém access log desligado, descarta error log
na location `/api/v1/invites/**`, remove `Referer` e `Forwarded`, e sobrescreve
`X-Forwarded-For`/`X-Real-IP` com `$remote_addr`. O IP estático do web é
`172.30.0.10`, exatamente o único CIDR confiável local (`/32`). Web e
PostgreSQL publicam somente em `127.0.0.1`.
Quando `LOCAL_DEMO_PROXY_LOOPBACK=true`, esse bind é uma restrição de segurança:
a porta web deve permanecer em `127.0.0.1` e não pode ser publicada em
`0.0.0.0` nem em outra interface acessível pelo host, pois o proxy renderizado
confia a identidade fixture exclusivamente ao caminho loopback.

No modo nativo, API e Vite usam somente os loopbacks
`127.0.0.1/32,::1/128`. `COMPOSE_TRUSTED_PROXY_CIDRS` é separado de
`TRUSTED_PROXY_CIDRS` para que copiar `.env.example` não substitua a confiança
exata no container web.

Os defaults de Compose e `.env.example` são fixtures deliberadamente públicas,
únicas por finalidade e exclusivas de `local`/`test`. Produção deve fornecer
URLs, origins, TTLs, limites e keyrings independentes por secret manager;
nenhuma fixture local pode ser reutilizada. Na Fase 3, HMAC de capability e
AES-GCM-256 usam material distinto dos cinco keyrings da Fase 2; o protótipo
também exige cleanup e TTL máximo de 24 horas.

Na Fase 4, `PUBLIC_DATABASE_URL` é obrigatório e distinto de `DATABASE_URL`
para o processo API. O login público lê exclusivamente as views
`public_data.current_summary`, `current_presence`, `current_preferences` e
`current_methodology`. `make phase4-remediation` combina código gerado, guardas
de build, seed fresh/persistente, full-stack e navegador real. Todos esses
targets continuam sendo provas locais de protótipo, não autorização
operacional.

Staging e produção exigem secret manager, PostgreSQL gerenciado, TLS, backup,
restore e provedor OIDC reais. Esses itens não são entregues pela Fase 1.
CIDRs de proxy de staging/produção também precisam ser explícitos, mínimos e
independentes da fixture local.

O target `make local-restore-drill` é uma prova mecânica local. Ele não declara
PITR, RPO de 15 minutos, RTO de 4 horas, política de retenção/custódia,
reaplicação de exclusões nem exercício institucional. Esses requisitos
continuam bloqueados até existirem provedor, responsáveis e evidências reais.

## Ambientes, semeadura e hot reload

Três stacks locais, todas sobre `compose.yaml`:

- `make up`: stack estática comprovada em `127.0.0.1:4173`, com Nginx servindo o
  build e as fixtures do `local-demo`. PostgreSQL publica em
  `127.0.0.1:${POSTGRES_HOST_PORT:-5433}`;
- `make docker-dev`: stack de hot reload em `127.0.0.1:5173`, projeto Compose
  `cumuru-dev`, subnet `172.30.8.0/24` e banco não publicado, para conviver com
  a estática. O web roda o Vite com HMR sobre `apps/web` montado; API e worker
  usam `apps/api/scripts/dev.sh`, que compara checksums a cada dois segundos,
  recompila e reinicia o processo. Uma compilação que falha preserva o processo
  anterior. `make docker-dev-logs DOCKER_DEV_SERVICES="api web"` acompanha;
- `deploy/compose.production.yaml`: runtime remoto, sem banco local.

`make docker-rm` derruba as duas stacks locais **apagando os volumes**, banco
incluído; `make docker-renew` faz isso e reconstrói as imagens antes de subir de
novo. Ambos são destrutivos e só recuperam o que o seeder e as fixtures sabem
recriar.

A stack de produção não é executável nesta máquina, e isso é deliberado:
`APP_ENV=production` exige `sslmode=verify-full`, `OIDC_MODE=real` com discovery
no boot, endpoint OTLP e origens HTTPS. Em vez de afrouxar essas regras para
caber num laptop, `make prod-config-check RUNTIME_ENV=<arquivo>` carrega o
arquivo pelos loaders reais de `api`, `worker` e `seed` e falha no primeiro
valor recusado, sem abrir socket nem tocar no banco. `deploy/runtime.env.example`
documenta as chaves; a mesma checagem roda dentro do container por
`/app/configcheck`. O arquivo é lido pelo próprio binário, e não por `source`,
porque o shell quebraria a DSN no `&` que separa `sslmode` de `sslrootcert` e
validaria um valor truncado.

O binário `/app/seed` faz a semeadura de bootstrap dos três ambientes e é
governado por duas variáveis:

- `SEED_ENABLED`: verdadeiro por padrão em `local|test`, falso em `staging|production`;
- `SEED_PROFILE`: `admin+demo` por padrão em `local|test`, `admin` nos demais.
  `admin+demo` é recusado fora de `local|test`; `none` deixa o seeder inerte.

Com um perfil ativo o seeder exige `SEED_ADMIN_EMAIL` e `SEED_ADMIN_PASSWORD` e
falha fechado sem eles, em vez de criar conta com credencial adivinhável. Fora
de `local|test` a conta nasce com `password_must_change`, e
`SEED_ADMIN_MUST_CHANGE_PASSWORD=false` é recusado ali: enquanto a marca existir
a sessão só alcança `POST /api/v1/auth/password`, `POST /api/v1/auth/logout` e
`GET /api/v1/auth/session`; qualquer rota com escopo responde 401. A troca
revoga todas as sessões abertas com a senha anterior, inclusive a que pediu a
troca.

Reexecutar o seeder é idempotente e **não** repõe a senha de bootstrap: o
`ON CONFLICT` da conta atualiza apenas nome e escopos. Os estabelecimentos vêm
de um catálogo versionado apontado por `SEED_ACCOMMODATION_CATALOG`, cujo
modelo é `seeds/accommodations.example.json`. O arquivo declara os próprios
identificadores, então uma reexecução atualiza as mesmas linhas em vez de criar
estabelecimentos duplicados; o decodificador é estrito e recusa chave
desconhecida, categoria fora da constraint e identificador repetido.

## Infraestrutura e deploy

- `terraform/bootstrap-state`: backend S3/KMS do state;
- `terraform/aws`: baseline AWS São Paulo com VPC, EC2, RDS, ECR, S3, KMS,
  Secrets Manager, alertas e orçamento;
- `ansible`: configuração idempotente e deploy da VM;
- `compose.production.yaml`: runtime remoto sem banco local;
- `compose.observability.yaml`: Collector, Prometheus, Tempo e Grafana locais;
- `make infra-validation`: valida Terraform, Ansible, ambos os Compose e
  referências OCI imutáveis; o target é obrigatório no CI local e remoto.

A decisão, custos, limites e fluxo seguro estão em
`docs/decisoes/ADR-031-infraestrutura-economica-aws-sao-paulo.md` e
`docs/12-infraestrutura-e-deploy.md`. Todas as imagens de terceiros ficam
fixadas por `tag@sha256`; o deploy das imagens próprias também exige os digests
retornados pelo ECR, conforme ADR-033. O manifesto de registry aceita somente
o `RepoDigest` que corresponda exatamente ao repositório e digest solicitados.
