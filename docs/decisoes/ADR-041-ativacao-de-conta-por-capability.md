# ADR-041 — Ativação de conta de acomodação por capability de uso único

**Status:** aceito para `PROTOTYPE_ONLY`.

**Relacionada:** [ADR-035](ADR-035-participacao-local-sem-cnpj-e-onboarding-prototipo.md)
para o autoprovisionamento, e
[ADR-037](ADR-037-autenticacao-local-por-email-e-senha.md) para a trilha local
de senha.

## Contexto

O produto descreve um administrador que cadastra a pousada e envia um link de
acesso. Duas coisas impedem transcrever isso literalmente.

Não existe papel de administrador provisionador. A ADR-035 implementou
autoprovisionamento: o próprio principal cria a acomodação, ela nasce `active`
e ele vira `manager`. Inventar um provisionador aqui contradiria uma decisão
aceita e recente.

Não existe envio de e-mail no projeto. Nenhum código SMTP, nenhum provedor.
Introduzi-lo nesta fase traria configuração, bounce, política de retry e uma
superfície nova de phishing, tudo fora do escopo do que a fase precisa provar.

Além disso, `auth.accounts` hoje impede representar conta pendente:
`password_hash` é `NOT NULL` com `CHECK (password_hash LIKE '$argon2id$%')`, e
`status` aceita apenas `active` ou `disabled`.

## Decisão

- a fase **não** cria papel de administrador provisionador e **não** envia
  e-mail. A capability de ativação é exibida na própria tela, como link e como
  QR gerado no navegador, sem serviço de terceiro;
- `auth.accounts` ganha o estado `pending_activation`;
- o CHECK do algoritmo passa a ser condicional a `password_hash IS NOT NULL`,
  em vez de condicional ao status;
- `accounts_credential_state_valid`, novo, amarra a ausência de hash
  exclusivamente a `pending_activation`. Sem essa segunda metade, relaxar a
  primeira permitiria conta `active` sem credencial;
- a capability é de uso único, revogável, armazenada por HMAC e nunca
  reconstruível sem o keyring, no mesmo desenho dos convites;
- conta pendente não autentica. A ativação define a senha e só então a conta
  passa a `active`.

## Consequências

- O fluxo desejado é preservado sem contradizer a ADR-035: quem provisiona
  continua sendo o principal, e a capability serve para transferir o acesso a
  quem opera a pousada.
- O envio por e-mail continua possível numa fase futura, reaproveitando
  `platform.outbox_events` como transporte, sem retrabalho do estado da conta.
- Enquanto não houver e-mail, a entrega do link é responsabilidade humana, fora
  do sistema. Isso é limitação declarada, não controle de segurança.
