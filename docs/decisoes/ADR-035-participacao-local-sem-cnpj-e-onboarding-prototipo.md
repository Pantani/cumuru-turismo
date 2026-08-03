# ADR-035 — Participação local sem CNPJ e onboarding do protótipo

**Status:** aceito para `PROTOTYPE_ONLY`.

## Contexto

O Observatório precisa representar pousadas formais, imóveis de temporada,
hospedagens familiares, campings e locais em regularização. O modelo já não
exige CPF, CNPJ ou Cadastur para uma acomodação e a criação de estadias não
depende da FNRH. Entretanto, a Fase 2 aceitava somente acomodações previamente
provisionadas. Um principal autenticado sem vínculo recebia uma lista vazia e
não conseguia iniciar a participação local.

Também havia dois riscos de interpretação: `category` aceitava texto livre que
chegava à cobertura analítica interna, e `cadastur_id` podia parecer um
ativador da FNRH embora a Fase 5 e seus gates externos estejam bloqueados.

## Decisão

O produto terá dois trilhos independentes:

1. **Observatório local:** funciona sem CPF, CNPJ, Cadastur ou chave FNRH para
   toda acomodação operacionalmente ativa.
2. **FNRH opcional:** permanece fora da Fase 2. Só poderá ser habilitada na
   Fase 5 depois de autorização, documentação vigente, homologação, mapeamento
   aprovado e credencial própria do estabelecimento.

Informar categoria ou Cadastur nunca prova enquadramento jurídico, regularidade,
autorização ou posse de credencial e nunca dispara integração. O Observatório
não gera a chave oficial da FNRH.

### Onboarding local

A Fase 2 adiciona `POST /accommodations`, protegido por OIDC, escopo
`accommodations:onboard`, `Idempotency-Key` e flag de ambiente. O endpoint:

- fica habilitado somente em `local|test`, com `PROTOTYPE_ONLY` e fake OIDC;
- deriva issuer e subject exclusivamente do token validado;
- não aceita organization ID, membership ID, CPF, CNPJ, Cadastur, contato,
  senha gov.br, chave FNRH ou flag de elegibilidade;
- recebe nome operacional, categoria fechada, capacidade e UUIDv7 de
  submissão;
- para um principal sem vínculo, cria organização mínima, acomodação e primeira
  membership `manager` na mesma transação;
- para um manager ligado a exatamente uma organização, cria outra acomodação
  nessa organização e a membership correspondente;
- recusa contexto ambíguo, ausência de autoridade e repetição divergente sem
  revelar outro tenant;
- grava idempotência, audit e outbox mínimos na mesma transação.

`active` significa somente que a jornada local está operacional no protótipo.
Não significa aprovação municipal, Cadastur ativo ou elegibilidade FNRH.
Onboarding com dados reais continua bloqueado até a Fase 0 definir autoridade,
IdP, aviso, base legal, revisão e operação.

### Categoria e Cadastur

`category` passa a aceitar somente códigos amplos:

```text
formal_lodging
seasonal_rental
family_hosting
camping
regularizing
other
unclassified
```

Os códigos são classificação operacional e não parecer jurídico. A migration
nova preserva a baseline `000001`, converte somente fixtures reservadas e falha
fechado diante de categoria antiga desconhecida; não faz inferência sobre dado
real. Entrada nova nunca aceita `unclassified`.

`cadastur_id` continua opcional em leitura para registros provisionados, mas
deixa de ser editável pela API genérica enquanto o formato e a verificação
oficial não estiverem congelados. O onboarding não recebe esse campo. Fixtures
podem usar valor claramente fictício para demonstrar a diferença visual, sem
criar integração.

Os grants de runtime deixam `document_hmac` fora das leituras comuns. Esta
remediação não coleta nem persiste documento fiscal.

## Consequências

- pessoa física, casa de temporada e hospedagem familiar conseguem iniciar e
  operar o fluxo local no protótipo sem CNPJ;
- pousada formal usa o mesmo fluxo local e não fica dependente da FNRH;
- categoria deixa de transportar texto identificável para analytics;
- contrato, migration, sqlc, Go, cliente TypeScript, React, changelog e testes
  mudam juntos;
- a Fase 5, a chave oficial, o onboarding real, dados reais, piloto, deploy e
  release permanecem `BLOCKED`.

