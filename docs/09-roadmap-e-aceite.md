# Estado, roadmap e critérios de aceite

Este documento descreve o que já existe, o que falta e como cada parte é
aceita. A organização é por **funcionalidade**, não por ordem de entrega: a
sequência em que cada onda foi implementada vive apenas no harness
(`prompts/BOOTSTRAP-CODEX.md`).

## Panorama

| Funcionalidade | Estado | Bloqueio remanescente |
| --- | --- | --- |
| `platform` — fundação técnica | implementada, gate local `PASS` | infraestrutura real `UNVERIFIED` |
| `core` — hospedagens e estadias | implementada, gate local `PASS` | base legal para dados reais |
| `questionnaire` — questionário e pesquisa | implementada, gate local `PASS` | base legal para dados reais |
| `analytics` — indicadores e painéis | implementada, gate local `PASS` | política de publicação aprovada |
| `self-service` — autocadastro e aprovação | implementada, gate local `PASS` | `SELF_SERVICE_LEGAL_BASIS` |
| `fnrh` — integração federal | não iniciada | autorização, docs oficiais e homologação |
| piloto controlado | não iniciado | depende de todos os gates acima |

Gate local `PASS` significa que `make ci` — a mesma cobertura que
`.github/workflows/ci.yml` exige na branch principal — prova contrato, banco,
privilégios, runtime HTTP, front-end e navegador real com dados fictícios. Não significa
autorização de deploy, release ou uso com dados de titulares reais. Todo o
runtime opera como `PROTOTYPE_ONLY`.

## Governança e descoberta

Pré-condição de qualquer uso real. Sem essas decisões, o sistema permanece
protótipo com dados fictícios.

Entregas esperadas:

- patrocinador e controlador definidos;
- mapa de hospedagens e capacidade;
- fluxo atual da FNRH;
- parecer jurídico;
- inventário de dados;
- RIPD inicial;
- política de publicação;
- seleção do provedor OIDC e da infraestrutura.

Estado: nenhuma dessas evidências foi produzida no repositório, e nenhum agente
ou documento pode inventá-las. Detalhes em
[`06-legal-e-governanca.md`](06-legal-e-governanca.md).

## `platform` — fundação técnica

Entregas: monorepo; API Go, worker e React; configuração por ambiente;
PostgreSQL e migrações; OIDC; OpenAPI e cliente gerado; logs, traces, métricas
e health checks; CI, SBOM e scanners.

Aceite:

- o ambiente sobe do zero com um comando documentado (`make up`);
- migrações aplicam e revertem conforme política;
- nenhuma rota autenticada aceita token inválido;
- o teste de isolamento por organização passa;
- o cliente OpenAPI é reproduzível (`make generated-check`);
- logs não carregam payload pessoal.

## `core` — hospedagens e estadias

Entregas: hospedagens, usuários e papéis; estadia, grupo e visitantes;
convite e QR Code; rascunho offline; check-in, check-out, cancelamento e
no-show; idempotência e concorrência otimista; auditoria; autenticação local
por e-mail e senha.

Aceite:

- repetir qualquer submissão não duplica visitante e reproduz a resposta;
- a mesma chave de idempotência com corpo diferente retorna `409`;
- `ETag`/`If-Match` divergente retorna `412`;
- operador da organização A não acessa a organização B;
- convite é persistido como HMAC, nunca em texto puro;
- transições inválidas de estado são recusadas;
- cancelado e no-show ficam fora da presença;
- alteração de saída recalcula os dias em `[entrada, saída)`;
- rascunho offline usa IndexedDB, nunca `localStorage`;
- mutação, outbox e auditoria são atômicas.

## `questionnaire` — questionário e pesquisa

Entregas: editor; revisão de privacidade; versões imutáveis; perguntas
condicionais; resposta separada da estadia; consentimentos versionados.

Aceite:

- o ciclo `draft → privacy_review → approved → published → retired` é
  respeitado;
- pergunta publicada não pode ser alterada; editar clona nova versão;
- nova versão não muda a semântica de respostas antigas;
- a DSL rejeita código, regex arbitrária e referência externa;
- pergunta sensível não é publicável sem aprovação;
- recusar a pesquisa não impede o check-in;
- consentimento é específico e versionado;
- texto livre não entra em analytics nem em dimensão pública.

## `analytics` — indicadores e painéis

Entregas: presença diária; cobertura; cubos permitidos; supressão e
arredondamento; painel público com metodologia; painel interno de qualidade.

Aceite:

- a reconciliação é idempotente e produz o mesmo resultado duas vezes;
- o papel `public_runtime` lê somente o schema `public_data`;
- nenhum payload público contém IDs internos, estabelecimento ou texto livre;
- células pequenas e complementares são suprimidas, com defesa contra
  differencing;
- o arredondamento é estável;
- o painel mostra cobertura e instante de atualização;
- observado e previsto são visualmente distintos;
- a última versão válida continua servida durante falha do agregador.

## `self-service` — autocadastro e aprovação

Entregas: convite reutilizável por acomodação; cartaz e QR do estabelecimento;
formulário aberto sem operador; fila de aprovação; ativação de conta por
capability.

Aceite:

- a capability de ativação é de uso único, revogável e não reconstruível sem o
  keyring;
- conta pendente não autentica antes da ativação;
- o convite reutilizável é revogável e a rotação invalida o token anterior;
- o token trafega no fragmento da URL e nunca em log, trace, métrica, audit ou
  outbox;
- o canal aberto rejeita nome, documento, e-mail, telefone e `role='minor'`;
- a estadia autocadastrada nasce sem membership autora e com proveniência
  `self_service`;
- estadia não aprovada não aparece em `analytics.presence_days` nem em
  `public_data`;
- aprovação e rejeição são idempotentes, respeitam `If-Match` e exigem operação
  própria;
- rejeição exige motivo de lista fechada e é auditada sem valor pessoal;
- rejeição e expiração eliminam os dados do autocadastro;
- a pendência expira em 72 horas, com auditoria;
- rate limit e proof-of-work respondem `429` com `Retry-After`, sem serviço de
  terceiro e sem cookie;
- isolamento A/B do convite reutilizável entre acomodações.

Gate externo pendente: a coleta em canal aberto, sem operador identificado e
sem contato com o titular, depende de base legal declarada
(`SELF_SERVICE_LEGAL_BASIS`). Sem ela, a funcionalidade só opera com dados
fictícios.

## `fnrh` — integração federal

Não implementada. Depende de autorização formal, documentação oficial vigente,
ambiente de homologação, regra aprovada de credencial por estabelecimento e
mapeamento de campos aprovado.

Entregas previstas: cofre de credenciais; adaptador para a versão vigente;
mapeamento de domínios; retry e dead letter; reconciliação; telas de status
para a hospedagem.

Aceite:

- a credencial nunca aparece em log ou leitura posterior;
- timeout não perde o registro local;
- reenvio não cria efeito duplicado;
- falha de validação é acionável e sanitizada;
- o contrato é validado em homologação oficial;
- cada estabelecimento usa a própria credencial.

## Piloto controlado

Não é executável só por código.

Escopo sugerido:

- 5 a 10 hospedagens de perfis diferentes;
- dados reais somente após os gates legais;
- 30 a 60 dias;
- suporte e canal do titular ativos;
- reunião semanal de qualidade.

Critérios para ampliar:

- adesão e cobertura mínimas;
- erro de duplicidade abaixo da meta;
- previsão com erro acompanhado;
- nenhum incidente crítico;
- tempo de preenchimento aceitável;
- restauração e resposta a incidente simuladas;
- parecer do comitê de governança.

## Backlog posterior

- alertas de demanda para comerciantes;
- API aberta de agregados;
- integração com calendários de eventos e clima;
- importação por PMS adicionais;
- pesquisa pós-visita;
- modelo preditivo avançado;
- acessibilidade em idiomas adicionais;
- portal de transparência e relatórios periódicos.

## Definition of Done

Uma história está pronta quando:

- requisito e ameaça foram considerados;
- contrato e migração estão atualizados;
- autorização negativa está testada;
- repetição e concorrência estão testadas;
- logs não vazam conteúdo;
- acessibilidade foi verificada;
- teste automatizado passa;
- `make post-task-quality` termina com exit code zero e emite
  `POST_TASK_QUALITY=PASS`;
- documentação operacional existe;
- o resultado foi demonstrado com dados fictícios;
- itens não verificados estão explícitos.
