# ADR-017 — Papéis e provisionamento da Fase 2

**Status:** aceito.

## Contexto

A Fase 2 precisa provar hospedagens, usuários, papéis e isolamento por tenant.
O modelo existente vincula um principal OIDC a uma acomodação, mas não existe
autoridade municipal formalmente definida para criar organizações, aprovar
hospedagens ou executar transições administrativas. Inventar esse ator faria o
protótipo simular uma decisão institucional ausente.

## Decisão

Nesta execução `PROTOTYPE_ONLY`:

- organizações e acomodações são previamente provisionadas somente por
  fixtures de integração/local com dados fictícios;
- a API não cria organizações ou a primeira acomodação e não executa aprovação
  municipal;
- o catálogo de membership contém somente `operator` e `manager`;
- `operator` pode ler e operar estadias da própria acomodação;
- `manager` possui as mesmas permissões, pode alterar os campos operacionais da
  acomodação e criar, alterar ou inativar memberships da própria acomodação;
- a API impede que a última membership `manager` ativa seja inativada ou
  rebaixada;
- issuer e subject do **ator da requisição** vêm somente do token validado;
  issuer e subject da **membership alvo** podem ser informados no body pelo
  manager autorizado, como dados operacionais fictícios, mas nunca ampliam a
  autoridade da própria requisição antes do vínculo ser persistido;
- toda query de domínio limita o recurso pelo par validado
  `(issuer, subject)`, pela membership ativa e pela acomodação;
- uma acomodação `active` aceita todas as jornadas;
- `suspended` nega nova estadia, convite, submissão de grupo e check-in, mas
  permite leitura, check-out, cancelamento e no-show de estadias existentes
  para encerrar obrigações já iniciadas;
- `closed` é somente leitura nesta fase;
- o status da acomodação não é mutável pela API desta fase.

A proteção da última `manager` ativa usa lock/condição no PostgreSQL dentro da
mesma transação da alteração. Duas tentativas concorrentes de rebaixamento ou
inativação não podem deixar a acomodação sem manager.

O contrato implementa listagem/leitura/alteração de acomodações já
provisionadas e listagem/criação/alteração de memberships. Não implementa
convite de usuário por e-mail, senha, IdP, papel municipal global, aprovação,
fiscalização ou recurso.

## Consequências

- O isolamento e a administração local de operadores são testáveis sem criar
  autoridade municipal fictícia.
- Cadastro e aprovação reais de hospedagens permanecem `BLOCKED` até definição
  formal de responsáveis e workflow.
- Dados, subjects e nomes usados nos testes são fictícios; nenhum `PASS`
  técnico autoriza piloto ou produção.
