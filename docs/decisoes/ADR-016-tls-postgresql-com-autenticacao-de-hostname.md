# ADR-016 — TLS PostgreSQL com autenticação de hostname

**Status:** aceito.

## Contexto

A configuração inicial rejeitava PostgreSQL sem TLS em `staging` e
`production`, mas ainda aceitava `sslmode=require` e `sslmode=verify-ca`.
O primeiro cifra sem autenticar o servidor; o segundo valida a cadeia da
autoridade certificadora sem exigir que o certificado pertença ao hostname
conectado. Ambos deixam uma lacuna incompatível com startup fail-closed fora
dos ambientes descartáveis.

Local e teste usam PostgreSQL efêmero em rede local do Compose, com identidades
e credenciais fictícias. Exigir uma PKI local nessa superfície não acrescenta
prova sobre a topologia institucional ainda indisponível.

## Decisão

Quando `APP_ENV` for `staging` ou `production`, `DATABASE_URL` deve:

- usar URL `postgres://` ou `postgresql://` válida;
- declarar exatamente um parâmetro `sslmode`;
- definir esse parâmetro como `verify-full`, com a grafia canônica;
- ser rejeitada se o modo estiver ausente, duplicado, vazio, combinado ou for
  `disable`, `allow`, `prefer`, `require` ou `verify-ca`.

O driver continuará responsável por validar a cadeia de confiança e comparar o
hostname usando a configuração TLS fornecida pelo ambiente. Provisionamento da
CA, certificados, DNS e segredos permanece responsabilidade da infraestrutura
institucional e precisa ser comprovado antes de staging real.

`local` e `test` podem usar `sslmode=disable` ou omitir o parâmetro somente com
banco descartável, dados fictícios e credenciais locais. Essa exceção não se
estende a preview, staging, produção ou dados reais.

Erros de validação expõem apenas o nome `DATABASE_URL`; DSN, usuário, senha,
hostname e parâmetros não entram em logs.

## Consequências

- staging e produção autenticam o hostname do PostgreSQL, além de cifrar o
  canal;
- configuração de certificado incompleta falha antes de abrir listeners;
- a experiência local continua simples sem simular uma garantia de produção;
- infraestrutura, CA institucional e conexão gerenciada continuam
  `UNVERIFIED` enquanto não houver ambiente externo autorizado.
