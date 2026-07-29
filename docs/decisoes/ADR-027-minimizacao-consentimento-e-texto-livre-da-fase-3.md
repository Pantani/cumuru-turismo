# ADR-027 — Minimização, consentimento e texto livre da Fase 3

**Status:** aceito.

## Contexto

Respostas turísticas podem revelar preferências e alcançar crianças. Texto
livre pode conter dados sensíveis mesmo com aviso. Não existem KMS, retenção,
RIPD ou salvaguardas institucionais aprovados, e a Fase 3 não inclui analytics
nem publicação de dados.

## Decisão

Toda resposta da Fase 3 é group-level. A API, o domínio e as migrations não
oferecem caminho de escrita para `respondent_visitor_id`. Stay, accommodation,
visitante, idade exata, nome, documento e contato não entram no request ou na
resposta pública.

Perguntas possuem classificação, finalidade, retenção e metadados de
agregação explícitos. Perguntas `sensitive|secret` nunca são publicáveis no
MVP. Perguntas específicas de crianças e lógica baseada em pessoa, papel ou
idade são proibidas.

A DSL condicional é declarativa, não recursiva e limitada a um grupo raiz
`all` ou `any`, com no máximo dez condições. Operadores permitidos são
`equals`, `not_equals`, `in`, `contains` e `answered`. Uma condição só
referencia `stable_key` anterior da mesma versão. Regex, código, funções, URLs,
paths, referências externas e nesting são rejeitados.

Participação e consentimentos são independentes:

- `declined` exige zero respostas e zero consent decisions e registra somente
  a decisão de participação;
- `submitted` contém exatamente uma decisão para cada requirement da versão;
- decisões são `{purpose_code, notice_version, granted}`, sem evidência livre;
- purpose e notice coincidem exatamente com o snapshot;
- uma answer só é aceita quando a decisão do `question.purpose_code` é
  `granted=true`; decisão negativa torna a pergunta não aplicável e proíbe sua
  resposta, inclusive quando a pergunta seria required;
- a projeção pública expõe `question.purpose_code`, identificador não sensível
  necessário para aplicar no cliente a decisão por finalidade;
- decisão duplicada, ausente, extra ou notice divergente causa `422` e rollback
  total;
- não existem decisões pré-marcadas;
- registros são append-only e nunca são atualizados ou apagados pelo runtime;
- recusa não altera `core.stays`, presença, check-in ou check-out.

Texto livre é opcional, classificado como `personal`, com
`public_aggregation_allowed=false` e `analytics_key=null`. Sua política é
binária:

1. em `local|test`, chave AES-GCM-256 separada e válida permite armazenar
   somente ciphertext autenticado, nonce e key version; ou
2. sem cifra válida, publicação e submissão de `short_text|long_text` falham
   fechadas.

AAD vincula response, questionnaire version, question e key version. Não há
coluna plaintext, índice de busca, endpoint de leitura, decrypt administrativo,
exportação ou consumer de analytics. O aviso contra nome, contato, saúde,
religião, opinião política e dado de terceiro é exibido junto ao campo.

Cada answer livre recebe `erase_after`, limitado a 24 horas depois da criação
em `local|test`. Um cleanup idempotente apaga ciphertext, nonce e key version
quando o prazo expira, sem copiar conteúdo para audit/outbox. Perguntas
`short_text|long_text` só podem ser publicadas quando cifra e cleanup estão
habilitados; sem qualquer um deles o fluxo falha fechado. Não existe legal hold
no protótipo.

Rascunhos do navegador usam o wrapper cifrado de IndexedDB existente, com TTL,
sem capability, bearer, stay ou visitor ID. Sucesso, recusa, logout, expiração,
schema incompatível ou cifra adulterada removem draft e chave. O service worker
não intercepta nem cacheia `/api/`.

Prompt, help e labels são renderizados exclusivamente como texto. A UI não usa
`dangerouslySetInnerHTML`, `eval` ou execução da DSL, e testes com HTML/script
provam a fronteira junto à CSP.

A Fase 3 não lê nem escreve `analytics` ou `public_data`. Audit e outbox usam
DTOs fechados com IDs, estados, versões e nomes de campos; nunca prompt, opção,
DSL, resposta, consentimento ou capability. O catálogo global admite
`organization_id` nulo em audit, sem inventar tenant.

Grants do runtime sobre responses, answers e consent decisions são
INSERT-only. `SELECT`, `UPDATE` e `DELETE` permanecem negados ao runtime, e
roles pública, worker e privacy officer não recebem acesso.

## Consequências

- a Fase 3 pode ser provada apenas com fixtures fictícias;
- texto livre nunca possui fallback em claro;
- nenhuma preferência é individualizada para menor;
- semântica histórica é preservada para uma futura Fase 4;
- retenção real, KMS, direitos do titular, analytics, supressão, differencing,
  dashboards e dados reais continuam `BLOCKED`, `UNVERIFIED` ou fora de escopo.
