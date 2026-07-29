# ADR-011 — Harness Codex faseado e paralelo

**Status:** aceito.

## Contexto

O blueprint exige implementação de uma fase por vez, com gates verificáveis, e
o `prompts/BOOTSTRAP-CODEX.md` fornece seis prompts sequenciais. Ao mesmo tempo,
o trabalho de estudo, implementação por superfícies independentes e QA pode se
beneficiar de subagentes.

O diretório ainda não é um repositório Git. Portanto, worktrees, branches e
reversão por commit não estão disponíveis nesta etapa. Subagentes também
compartilham o mesmo filesystem, o que torna escrita concorrente em contratos,
migrações, arquivos gerados e configuração compartilhada especialmente
arriscada.

O skill `harness` originalmente descreve superfícies Claude em `.claude/`.
Codex, porém, descobre skills de projeto em `.agents/skills/` e agentes
customizados em `.codex/agents/*.toml`.

## Decisão

Adotar um harness híbrido com:

- `.agents/skills/cumuru-bootstrap/` como orquestrador principal reconhecido
  pelo Codex;
- `.agents/skills/cumuru-phase-orchestrator/` como motor de uma fase;
- `.agents/skills/cumuru-integration-qa/` como protocolo de QA incremental;
- `.codex/agents/*.toml` como definições operacionais de especialistas;
- `.claude/agents/` e `.claude/skills/` como camada de compatibilidade e
  documentação para o ecossistema descrito pelo skill `harness`;
- `AGENTS.md` como ponteiro persistente do Codex e `CLAUDE.md` como ponteiro
  compatível;
- `_workspace/cumuru-bootstrap/` como trilha de estudos, planos, tentativas e
  evidências.

O paralelismo seguirá estas regras:

1. estudos independentes podem usar até três subagentes simultâneos;
2. nenhuma implementação começa antes da síntese do estudo da própria fase;
3. fases de implementação são sequenciais;
4. dentro de uma fase, escritores só podem rodar em paralelo com ownership
   explicitamente disjunto;
5. contrato OpenAPI, migrações, código gerado, changelog e configuração
   compartilhada têm escritor único;
6. sem Git, o padrão é um único escritor por vez;
7. QA roda incrementalmente depois de cada slice e novamente no gate da fase.

Não fixar um modelo Claude como `opus` nas configurações Codex. Os agentes Codex
herdam o modelo da sessão, salvo solicitação explícita do usuário. A camada
`.claude` mantém `model: opus` apenas para compatibilidade com o harness
original.

## Gates

- Fases dependentes exigem `PASS` reproduzível da fase anterior.
- Fase 0 incompleta limita o sistema a protótipo com dados fictícios.
- Fase 5 fica `BLOCKED` se faltar autorização formal, documentação oficial
  vigente, homologação, regra de credencial por estabelecimento ou mapeamento
  aprovado.
- Fase 6 não autoriza deploy nem release.
- `FAIL`, `BLOCKED` e `UNVERIFIED` nunca são promovidos silenciosamente a
  `PASS`.

## Consequências

O estudo de fases futuras pode avançar em paralelo sem violar o roadmap, mas a
implementação preserva a ordem necessária para contratos, migrações e regras de
privacidade. O custo é maior disciplina de ownership e manutenção de duas
superfícies de descoberta, mitigada por wrappers mínimos em `.claude/` e por um
validador estrutural.
