# ADR-019 — Convites, QR e rascunho offline

**Status:** aceito.

## Contexto

O blueprint menciona link, QR, código curto, rascunho de grupo no servidor e
rascunho offline. Essas superfícies têm ameaças diferentes. Um QR público
reutilizável exigiria uma segunda prova ainda não definida; persistir rascunho
no servidor ampliaria retenção e PII sem necessidade para o gate técnico.

## Decisão

- cada convite pertence a uma estadia e usa capability determinística,
  não forjável, codificada em base64url sem padding:
  `invite_id_bytes || HMAC-SHA-256(key_version, purpose || invite_id_bytes)`;
- o banco guarda invite ID, HMAC do token, finalidade e versão da chave, nunca
  a capability completa; um dump sem o keyring não reconstrói o token;
- a versão histórica da chave permite reconstruir a mesma URL exclusivamente
  para o replay idempotente da criação;
- a URL completa é retornada na criação e em seu replay exato; o QR é gerado
  localmente no navegador, sem serviço de terceiros;
- o convite padrão do protótipo expira em 72 horas, aceita um uso e pode ser
  revogado; o prazo é configurável e não representa política institucional;
- reemissão revoga convites ainda ativos da estadia;
- token ausente, incorreto, expirado ou revogado retorna o mesmo `404`;
- submissão repetida com a mesma idempotency key e mesmo hash reproduz o
  sucesso; a mesma key com hash diferente retorna `409`; tentativa nova em
  convite já consumido retorna `409 invite-consumed`;
- token, URL e HMAC não entram em log, trace, métrica, audit, outbox ou cache;
  o idempotency record guarda apenas o invite ID necessário para reconstruir o
  replay;
- código curto e QR público reutilizável ficam fora da Fase 2 até existir
  segunda prova aprovada;
- não haverá `PUT /invites/{token}/group`; o servidor recebe somente a
  submissão final.

Os endpoints de convite têm limiter real persistido em PostgreSQL:

- contexto: 30 tentativas por minuto por capability derivada e IP
  generalizado;
- submissão: 10 tentativas por minuto pela mesma composição;
- o valor é configurável, o teste usa relógio/limites controlados e `429`
  inclui `Retry-After`;
- token e IP bruto nunca são persistidos ou logados.

Requests browser mutáveis aceitam somente origens da allowlist configurada.
Convites não usam cookies; JSON estrito e `Idempotency-Key` exigem preflight e
impedem form POST simples. CORS não reflete origem arbitrária.

O rascunho é local:

- IndexedDB guarda somente campos generalizados, schema version, UUIDv7 e
  timestamps;
- o payload é cifrado com AES-GCM por uma `CryptoKey` não exportável, persistida
  pelo próprio IndexedDB; nonce é único por gravação;
- `localStorage` e `sessionStorage` não são usados;
- token de convite, token OIDC, headers, nome, contato, documento e texto livre
  nunca são persistidos;
- sync acontece em primeiro plano depois de revalidar a capability;
- falha, `409`, `412` ou `422` preserva o rascunho; sucesso novo ou replay
  confirmado o remove;
- o TTL local técnico é 24 horas; convite expirado/revogado (`404` após uma
  capability antes válida), descarte explícito, logout/sessão encerrada,
  schema incompatível ou falha autenticada de cifra removem rascunho e chave;
- o record usa um draft ID aleatório; a aplicação pode guardar esse ID em
  fragmento local da URL, mas não persiste token, URL ou fingerprint do token;
- o service worker cacheia somente shell estático versionado e nunca intercepta
  ou cacheia `/api/`, convite ou resposta autenticada;
- a UI oferece descarte explícito e trata cifra local como defesa em
  profundidade, não como proteção contra XSS ou acesso ao mesmo perfil.

## Consequências

- O fluxo offline é testável sem criar retenção parcial no servidor.
- Reabrir o formulário exige novamente o link/capability; o token não é
  recuperável do IndexedDB.
- Logs de CDN/WAF, política real de expiração e uso em dispositivo real
  permanecem `UNVERIFIED/BLOCKED` antes do piloto.
