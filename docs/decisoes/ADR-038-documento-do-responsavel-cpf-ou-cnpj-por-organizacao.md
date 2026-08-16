# ADR-038 — Documento do responsável (CPF ou CNPJ) por organização

**Status:** aceito para `PROTOTYPE_ONLY`.

**Supersede parcialmente:** [ADR-035](ADR-035-participacao-local-sem-cnpj-e-onboarding-prototipo.md),
na parte em que o onboarding local "não aceita CPF, CNPJ" e "não coleta nem
persiste documento fiscal". Os dois trilhos da ADR-035 permanecem válidos: o
Observatório continua independente da FNRH, e este documento **não** é
credencial, prova de regularidade nem habilitador de integração.

## Contexto

A ADR-035 removeu qualquer documento do cadastro porque `cadastur_id` podia
parecer um ativador da FNRH e porque exigir CNPJ excluiria o público real do
Observatório: pessoas físicas alugando casa de temporada ou anunciando em
plataforma, sem empresa constituída.

O objetivo era correto, mas o efeito colateral foi perder qualquer âncora de
identidade do responsável. Sem ela:

- a mesma casa pode ser cadastrada duas vezes e inflar a contagem do
  observatório, que é justamente o produto entregue à cidade e aos comerciantes;
- não há como distinguir dois cadastros homônimos;
- não há vínculo verificável entre a acomodação e quem responde por ela.

`core.organizations` já previa `legal_name` e `document_hmac` desde a baseline,
sem nenhum caminho de escrita.

## Decisão

O cadastro de uma organização passa a exigir **exatamente um** documento do
responsável: **CPF ou CNPJ**, nunca ambos, nunca nenhum.

### Por que isso não exclui o público

Todo anfitrião pessoa física tem CPF. Pousada e hostel constituídos usam CNPJ.
A exigência é satisfeita por qualquer participante, formal ou informal, sem
Cadastur, sem alvará e sem conta gov.br. A ADR-035 continua valendo: nenhum
documento prova enquadramento jurídico, regularidade ou elegibilidade.

### Armazenamento: só HMAC, sem volta

O documento é normalizado, validado e convertido em HMAC com chave rotacionável
e separada da chave de dados pessoais, conforme
[`docs/03-modelo-de-dados.md`](../03-modelo-de-dados.md): *"Hash simples de CPF
não é aceitável; use HMAC com chave rotacionável e separada."*

**O valor em claro nunca é persistido.** Não há caminho de leitura, nem para
suporte, nem para administrador, nem para quem obtiver o dump do banco. A
aplicação sabe responder "este documento já está cadastrado" e nada além disso.

Consequência aceita conscientemente: é impossível exibir, imprimir, conferir ou
exportar o número depois do cadastro. Se algum dia surgir necessidade concreta
de leitura — contrato, prestação de contas ao Município — será outra ADR, com
cifra reversível, controle de acesso e auditoria de cada consulta.

### Unicidade: um documento, um cadastro

`document_hmac` recebe índice único. A tentativa de cadastrar um documento já
usado é recusada como conflito, sem revelar qual organização o detém — a
mensagem não pode virar um oráculo de "este CPF está no sistema".

Quem administra mais de uma propriedade cadastra **várias acomodações na mesma
organização**, não várias organizações. O fluxo já suporta isso: um manager
ligado a exatamente uma organização cria outra acomodação nela.

### Escopo da coleta

O documento pertence à **organização**, não à acomodação. É pedido apenas quando
o onboarding cria uma organização nova. Manager já vinculado que adiciona a
segunda casa não informa documento de novo.

### Validação

Dígitos verificadores de CPF e CNPJ são conferidos antes do HMAC. Documento
malformado é recusado na entrada, com erro genérico. Sequências repetidas
notoriamente inválidas são recusadas.

## Consequências

- o observatório ganha uma âncora contra duplicidade sem guardar dado legível;
- vazamento de banco não expõe nenhum CPF, porque não há CPF armazenado;
- o valor em claro é indevassável por design, e isso é permanente sem nova ADR;
- rotação da chave de HMAC invalida a comparação com registros antigos e exige
  procedimento próprio, ainda não definido;
- contrato OpenAPI, migration, sqlc, Go, cliente TypeScript, React, changelog e
  testes mudam juntos;
- o documento entra no inventário de dados pessoais e no prazo de retenção; a
  eliminação a pedido do titular remove o HMAC junto com a organização;
- a Fase 5, a chave oficial da FNRH, dados reais, piloto, deploy e release
  permanecem `BLOCKED`.

## Alternativas descartadas

| Alternativa | Motivo da recusa |
| --- | --- |
| Não coletar documento (status quo da ADR-035) | não resolve duplicidade, que corrompe o produto entregue à cidade |
| Exigir só CNPJ | exclui a maioria do público: pessoa física alugando casa |
| Guardar o número cifrado e reversível | custo de gestão de chave, controle de acesso e auditoria sem necessidade concreta hoje |
| Guardar hash simples (SHA-256 sem chave) | proibido pelo blueprint: CPF tem espaço pequeno e é quebrável por força bruta |
| Permitir duplicidade com alerta | exige revisão humana contínua que o piloto não tem |
