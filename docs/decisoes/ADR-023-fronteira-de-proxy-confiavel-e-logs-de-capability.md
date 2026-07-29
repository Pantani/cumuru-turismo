# ADR-023 — Fronteira de proxy confiável e logs de capability

**Status:** aceito.

## Contexto

O token de convite da Fase 2 é uma capability no path. Mesmo com access log
desligado, Nginx e Vite podem incluir a URI completa em erros de upstream,
especialmente durante indisponibilidade. Isso viola a decisão de nunca registrar
token ou URL de convite.

O rate limit público também depende do IP generalizado. Se o proxy preservar ou
anexar `Forwarded`, `X-Forwarded-For` ou `X-Real-IP` enviados pelo cliente, um
atacante pode escolher a identidade usada pelo limiter. Confiar toda a subnet
Compose ampliaria desnecessariamente essa fronteira.

## Decisão

### Topologia e confiança

- a API da aplicação não publica porta no host quando está atrás do web proxy;
- somente o web proxy expõe e encaminha a API ao host; os serviços internos
  continuam na rede privada Compose;
- o web proxy recebe IP privado estático, e `TRUSTED_PROXY_CIDRS` local contém
  exatamente esse endereço `/32`;
- no desenvolvimento nativo, API e Vite rodam no host e a allowlist contém
  somente `127.0.0.1/32` e `::1/128`; a variável Compose é separada para que um
  `.env` de desenvolvimento não substitua o `/32` do container web;
- cada override de teste usa subnet própria e endereço estático correspondente,
  evitando colisão com a stack local;
- produção e staging devem fornecer CIDRs próprios e mínimos; a configuração
  local nunca é promovida;
- o backend futuro só aceitará headers de encaminhamento quando o socket remoto
  pertencer à allowlist confiável. Até essa implementação serial, o smoke que
  dependa da interpretação desses headers é evidência interina.

### Headers de encaminhamento

Nginx sempre:

- remove `Referer` e `Forwarded`;
- sobrescreve `X-Forwarded-For` e `X-Real-IP` com `$remote_addr`;
- nunca anexa uma cadeia fornecida pelo cliente.

Vite dev/preview aplica a mesma política, usando exclusivamente
`request.socket.remoteAddress`. Valor recebido nos quatro headers é removido ou
sobrescrito antes do upstream.

### Logs de capability

- access log permanece desligado;
- a location específica `/api/v1/invites/**` descarta seu error log local,
  inclusive quando o upstream está indisponível;
- o logger customizado do Vite substitui mensagem de erro de proxy por texto
  constante e descarta dados estruturados de erro;
- testes com token sintético e upstream indisponível verificam stdout, stderr e
  logs dos dois proxies; encontrar o token ou URI é falha;
- não se adiciona allowlist ou suppression de scanner/log para esconder o
  problema.

### Service worker e cleanup

- assets de shell só podem entrar no cache quando pertencem a `/assets/` e
  `url.search === ""`;
- API, convite, request autenticada e método diferente de `GET` continuam fora;
- não há background sync;
- cleanup de projetos Compose preserva o exit principal quando ele falha, mas
  retorna falha quando o fluxo principal passou e a remoção falhou;
- o full-stack confirma ausência residual de containers, rede e volume.

## Consequências

- o rate limiter pode receber identidade de cliente somente após o backend
  validar `TRUSTED_PROXY_CIDRS`; configuração de plataforma isolada não prova
  essa etapa;
- health, readiness e rotas autenticadas locais são testadas pelo proxy web,
  não por binding direto da API;
- a CI ganha um full-stack efêmero com API, worker, web, PostgreSQL, migrations,
  timezone, proxy, service worker, rota F2 e canário de indisponibilidade;
- erros de capability no proxy perdem detalhe operacional de propósito; métricas
  agregadas e request IDs sem URI continuam sendo a superfície apropriada.
