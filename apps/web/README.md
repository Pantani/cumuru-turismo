# Web

SPA estática da fundação técnica do Observatório Turístico de Cumuruxatiba.

## Limite do protótipo

Questionário, dashboard e FNRH continuam fora desta fase. Dados e identidades
locais devem ser exclusivamente fictícios.

O ambiente opera como `PROTOTYPE_ONLY`, exclusivamente com dados fictícios.

## Desenvolvimento

Execute a partir da raiz do repositório:

```bash
npm --prefix apps/web run dev
npm --prefix apps/web run typecheck
npm --prefix apps/web run lint
npm --prefix apps/web run test
npm --prefix apps/web run build
```

O cliente usa `/api/v1` no mesmo origin por padrão. Durante
`npm --prefix apps/web run dev`, o proxy do Vite encaminha esse prefixo para
`http://127.0.0.1:8080`; o preview Vite e o Nginx de Compose fazem o mesmo
encaminhamento. Os proxies removem `Referer` antes do upstream e não habilitam
access log. Vite remove `Forwarded`, sobrescreve
`X-Forwarded-For`/`X-Real-IP` somente com `request.socket.remoteAddress` e usa
logger customizado para substituir erro de proxy por mensagem constante, sem
URI de capability, stack ou `options.error`.

Quando API e Vite rodam nativamente, `TRUSTED_PROXY_CIDRS` deve conter somente
`127.0.0.1/32,::1/128`; Compose usa o `/32` separado do container web.
`VITE_API_BASE_URL` é um override opcional para ambientes cuja API tenha uma
origem ou um prefixo diferente. O estado remoto é gerenciado pelo TanStack
Query. `qrcode` é a implementação local fixada para QR, e
`fake-indexeddb` existe somente como dependência de teste.

`public/service-worker.js` congela a estratégia shell-only. Ele trata somente
navegação e assets same-origin, nunca intercepta ou cacheia `/api/`, request
com `Authorization` ou URL de capability de convite, e não implementa
background sync. Assets sob `/assets/` só entram no cache com query vazia. O
registro desse asset pertence à feature frontend. Rascunhos offline usam
IndexedDB; `localStorage` e persistência de token são proibidos.

O arquivo `src/generated/schema.ts` é produzido a partir do OpenAPI pelo owner
de plataforma e nunca deve ser editado manualmente.

O lint usa `apps/web/.oxlintrc.json` e cobre todo arquivo TypeScript, TSX e
JavaScript próprio dentro de `apps/web`, incluindo `vite.config.ts`, testes e o
cliente gerado. `node_modules` e `dist` são os únicos artefatos fora dessa
medição. As regras `complexity` e `sonarjs/cognitive-complexity` falham quando
uma função ultrapassa 9, sem comentários de suppression.
