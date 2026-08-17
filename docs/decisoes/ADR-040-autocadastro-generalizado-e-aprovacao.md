# ADR-040 — Autocadastro generalizado e aprovação do estabelecimento

**Status:** aceito para `PROTOTYPE_ONLY`.

**Relacionada:** [ADR-039](ADR-039-convite-reutilizavel-por-acomodacao.md) para
o transporte, e [ADR-020](ADR-020-minimizacao-presenca-e-eventos-da-fase-2.md)
para a ausência de cofre de identidade.

## Contexto

O produto quer que qualquer pessoa possa iniciar um cadastro pelo QR do
estabelecimento, e que o estabelecimento aprove depois. A intenção inicial era
aceitar identidade completa nesse canal.

Três fatos mudaram a decisão.

`identity.visitor_identities` **não existe na cadeia executável**. Ela consta de
`database/schema.sql` como modelo lógico, mas a migração `000001` cria apenas
`CREATE SCHEMA identity`, sem tabela, e o único grant é
`GRANT USAGE ON SCHEMA identity TO privacy_officer`. A ADR-020 vetou a tabela
deliberadamente. Criá-la aqui significaria construir cifra de coluna com
versionamento de chave sem o KMS que `docs/05-privacidade-e-seguranca.md` exige.

O canal é aberto e não autenticado. Aceitar identidade nele permite que um
estranho crie um registro nominal falso sobre uma pessoa real num sistema
municipal. Nenhum controle técnico endereça isso: o atacante precisa de **uma**
submissão, não de mil, então rate limit e proof-of-work não tocam a ameaça. O
único controle seria a aprovação humana, que falha por fadiga.

O titular nunca é contatado, porque esta fase não envia e-mail. Ele não tem como
saber que um registro sobre ele existe.

## Decisão

- o canal `self_service` coleta **somente dados generalizados**: faixa etária,
  UF e município IBGE, país, papel no grupo e datas. Nome, documento, e-mail e
  telefone são rejeitados;
- `role='minor'` é recusado no canal aberto. Aceitar submissão sobre criança
  feita por estranho não autenticado é indefensável;
- a identidade, quando a finalidade exigir, é preenchida **pelo próprio
  titular** depois da aprovação, pelo convite nominal já existente da Fase 2.
  Não há segundo caminho de identidade;
- a estadia autocadastrada nasce com proveniência `self_service` e sem
  membership autora. `created_by_membership_id` passa a aceitar nulo, e
  `stays_provenance_author_valid` exige autora não nula quando
  `provenance = 'assisted'`, preservando a obrigação para toda linha da Fase 2
  através do `DEFAULT 'assisted'`;
- a espera de aprovação **não** é um valor novo em `core.stay_status`. É
  proveniência mais carimbo de aprovação;
- a aprovação exige operação própria em `allowedOperations`, disponível apenas
  na acomodação ativa. Reaproveitar `update_stay` daria aprovação a qualquer
  operador com permissão de edição;
- a rejeição exige motivo de **lista fechada**, no precedente de
  `questionnaire_change_reason_valid`. Texto livre em `platform.audit_events`,
  que é append-only sem `UPDATE` nem `DELETE`, viraria dado pessoal permanente;
- pendência não aprovada expira em **72 horas**, mesmo prazo do convite da
  Fase 2, para o produto não passar a ter duas noções de validade;
- **rejeição e expiração** eliminam os dados do autocadastro e preservam apenas
  o fato auditável. Eliminar somente na rejeição permitiria retenção indefinida
  por inação;
- estadia não aprovada não alcança `analytics.presence_days` nem `public_data`.
  O filtro tem três pontos obrigatórios: projeção em
  `ListPresenceReconciliationStays` sem tocar o `WHERE`, `presenceEligible()` em
  `analytics_repository.go`, e um predicado em
  `analytics.aggregate_eligible_preferences`;
- `RequestHash` passa a ser com chave, para o digest não virar oráculo de
  confirmação por palpite sobre dado já eliminado.

## Consequências

- O canal aberto deixa de carregar payload identificável, e a ameaça de
  registro difamatório perde objeto.
- O fluxo do produto é preservado: qualquer pessoa inicia, o estabelecimento
  aprova, e a identidade vem depois pelo caminho já aceito.
- A purga opera sobre `core.visitors` generalizado, e é provável por varredura
  de `information_schema.columns` nos dois caminhos.
- O caminho FNRH permanece aberto, porém sempre passando pelo convite nominal.
- Permanece `BLOCKED` para dados reais até `SELF_SERVICE_LEGAL_BASIS` estar
  `PASS`: coleta aberta sem operador identificado e sem contato com o titular
  continua sendo questão de base legal, mesmo generalizada.
