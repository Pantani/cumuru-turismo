# ADR-014 — Principal OIDC e isolamento de tenant

**Status:** aceito.

## Contexto

OIDC identifica um usuário pelo par estável `(issuer, subject)`. O schema lógico
inicial guardava apenas `oidc_subject`, que pode colidir entre provedores.
Claims de organização fornecidas pelo cliente também não constituem vínculo
interno confiável.

## Decisão

O principal interno mínimo contém:

- issuer normalizado e validado;
- subject;
- scopes normalizados;
- nenhum token bruto depois da verificação.

`core.memberships` passa a persistir `oidc_issuer` e `oidc_subject`, com
unicidade por acomodação e pelo par do principal. Organização e acomodação
autorizadas são resolvidas por query interna a partir de memberships ativas;
headers, body e claims de tenant não são autoridade.

Issuer, audience, assinatura, algoritmo e expiração são validados por
`github.com/coreos/go-oidc/v3/oidc` `v3.20.0`; `nbf` é validado adicionalmente
após a verificação criptográfica da biblioteca. A aplicação não implementa
assinatura JWT, discovery ou JWKS. Discovery/JWKS usa cliente HTTP com timeout
e falha fechado. O fake é uma implementação da mesma interface, selecionada
explicitamente apenas quando `APP_ENV` for `local` ou `test`; ele nunca é
fallback após falha do provedor.

Autenticação, escopo e tenant são gates separados:

- credencial ausente ou inválida: `401`;
- credencial válida sem escopo: `403`;
- tenant inexistente ou fora do vínculo: negação uniforme sem revelar a
  existência do recurso.

Logs, métricas e traces não recebem token, claims completos, sujeito nem
identificadores de tenant como labels.

## Consequências

- O schema lógico e a migration inicial precisam incluir `oidc_issuer`.
- Troca de provedor não mistura sujeitos homônimos.
- A Fase 1 pode provar isolamento A/B em um harness de integração interno, sem
  criar endpoint público artificial de domínio.
- Catálogo definitivo de papéis e fluxos de membership permanece para a Fase 2.
