# ADR-037 — Autenticação local por e-mail e senha

**Status:** aceito para `PROTOTYPE_ONLY`. Complementa o
[ADR-035](ADR-035-participacao-local-sem-cnpj-e-onboarding-prototipo.md).

## Contexto

O ADR-035 removeu a exigência de CNPJ, Cadastur e chave FNRH do modelo de
dados, mas não do acesso. A única credencial aceita era um token OIDC, e na
demonstração local esse token era injetado no bundle em tempo de build por
`VITE_LOCAL_DEMO_IDENTITY`. Consequências práticas:

- não existia tela de entrada: quem abria `/acesso` sem o build de demonstração
  via apenas "o provedor OIDC institucional não entregou uma sessão";
- o Município de Prado não tem IdP definido — a Fase 0 ainda não decidiu
  autoridade nem provedor —, então a trilha OIDC não tinha como ser usada por
  uma pousada ou casa de temporada;
- um build de demonstração carregava uma credencial dentro do JavaScript
  servido.

## Decisão

Adotar autenticação local por e-mail e senha, coexistindo com OIDC.

### Credencial

- `auth.accounts` guarda a codificação Argon2id completa (PHC), com sal por
  conta. Os parâmetros seguem a configuração do OWASP Password Storage Cheat
  Sheet: 19 MiB, duas iterações, uma lane;
- o segredo tem no mínimo 12 caracteres, seguindo a NIST SP 800-63B ao preferir
  comprimento a regras de composição;
- e-mail desconhecido, senha incorreta e conta desabilitada respondem o mesmo
  401. Só o bloqueio temporário se distingue, com 429 e `Retry-After`;
- cinco tentativas falhas bloqueiam a conta por quinze minutos.

### Sessão

- `POST /auth/login` devolve um token opaco prefixado por `cms_`. O servidor
  guarda apenas o SHA-256 desse token em `auth.sessions`;
- o token vive somente na memória da aba: nunca em `localStorage`,
  `sessionStorage`, cache do service worker ou árvore de render. Recarregar a
  página encerra a sessão, por construção;
- a janela ociosa desliza a cada uso, limitada pelo prazo absoluto;
- `POST /auth/logout` revoga do lado do servidor e é idempotente.

### Coexistência com OIDC

`access.LocalSessionIssuer` ocupa o mesmo campo que um issuer OIDC, e o subject
é o UUID da conta. Uma conta local alcança `core.memberships` pelo mesmo
caminho de autorização que um principal federado — não há segunda trilha de
permissão.

O despacho é determinístico pelo prefixo do token: um token de sessão nunca
chega ao verificador OIDC e um token OIDC nunca consulta a tabela de sessões.
Quando a Fase 0 definir o IdP municipal, as duas trilhas convivem sem
alteração no modelo de autorização.

### Remoção da identidade em build time

`VITE_LOCAL_DEMO_MODE` e `VITE_LOCAL_DEMO_IDENTITY` foram removidos do
`vite.config.ts`, do Compose, do Makefile e dos testes. O binário e o bundle
não carregam credencial. A conta de demonstração é semeada pelo `local-demo` a
partir de `LOCAL_DEMO_ACCOUNT_PASSWORD`, que falha fechado se ausente.

## Consequências

- pousada, casa de temporada, quarto familiar, camping e local em regularização
  entram com e-mail e senha, sem CNPJ, Cadastur ou chave federal;
- o segredo nunca aparece em log, métrica, trace ou evento de auditoria;
- os grants deixam `password_hash` fora de `worker_runtime`, `public_runtime` e
  `privacy_officer`;
- a sessão é bearer: copiada, vale até expirar. Amarrá-la ao dispositivo
  exigiria prova de posse, fora do escopo do protótipo;
- dados reais, piloto, deploy e release permanecem `BLOCKED`; esta decisão vale
  para o protótipo local.
