# Domínios e fluxos

## Estados da hospedagem

```text
PENDING_REVIEW -> ACTIVE -> SUSPENDED -> CLOSED
```

Uma hospedagem suspensa preserva dados e histórico, mas não cria novas estadias.

## Estados da estadia

```text
DRAFT -> INVITED -> PRE_REGISTERED -> CHECKED_IN -> CHECKED_OUT
                     |                  |
                     +-> CANCELLED      +-> CANCELLED (somente correção autorizada)
                     +-> NO_SHOW
```

Regras:

- `CHECKED_IN` exige pelo menos um visitante válido;
- `CHECKED_OUT` exige data/hora de saída real;
- `CANCELLED` e `NO_SHOW` não alimentam presença;
- mudança retroativa após fechamento gera evento de correção;
- data de saída não pode preceder entrada;
- estadias sobrepostas do mesmo identificador pseudônimo geram alerta, não
  bloqueio automático.

## Criação pelo operador

1. Operador cria rascunho com entrada, saída e quantidade prevista.
2. API retorna link de uso único e QR Code.
3. Responsável abre o link, confirma o grupo e aceita o aviso de privacidade.
4. Preenche integrantes e envia.
5. API valida, deduplica e confirma.
6. Pesquisa opcional é oferecida em nova etapa.
7. No check-in, operador confirma presença.
8. Worker envia à FNRH quando configurado e permitido.

## Autocadastro sem operador

Permitido apenas quando a hospedagem disponibiliza QR Code associado e o
responsável confirma um código emitido pelo estabelecimento. QR Code público sem
segunda prova facilitaria registros falsos.

## Cadastro assistido

O operador pode registrar o grupo, mas:

- deve informar quem forneceu os dados;
- deve exibir ou entregar o aviso de privacidade;
- não pode marcar consentimento opcional em nome do hóspede;
- deve entregar código para revisão;
- toda ação fica auditada.

## Rascunho offline

1. O navegador gera `submission_id` UUIDv7.
2. O rascunho cifrado localmente fica no IndexedDB.
3. Ao recuperar conexão, envia com `Idempotency-Key` derivada de
   `submission_id`.
4. Se o conteúdo divergir de uma submissão já aceita, a API responde `409`.
5. Após confirmação, o navegador apaga o rascunho local.

Não guardar tokens longos nem documentos completos para sincronização em
segundo plano.

## Questionários

Estados:

```text
DRAFT -> PRIVACY_REVIEW -> APPROVED -> PUBLISHED -> RETIRED
```

Regras:

- apenas rascunhos são editáveis;
- publicação congela conteúdo, opções e finalidade;
- uma nova edição clona a versão;
- pergunta sensível ou livre exige revisão de privacidade;
- toda pergunta tem classificação, base/finalidade, obrigatoriedade e política
  de agregação;
- pergunta opcional sem resposta não vira valor “não”;
- consentimento promocional nunca é condição da estadia.

Tipos iniciais:

```text
short_text
long_text
single_choice
multiple_choice
boolean
integer_range
rating
date
state_city
```

## Regras condicionais

Use uma DSL JSON limitada:

```json
{
  "all": [
    {"question": "travel_reason", "operator": "contains", "value": "event"},
    {"question": "age_band", "operator": "in", "value": ["18_24", "25_34"]}
  ]
}
```

Operadores permitidos: `equals`, `not_equals`, `in`, `contains`, `answered`.
Sem código executável, regex arbitrária ou acesso a outra pessoa.

## Direitos do titular

1. Titular informa código da estadia e valida o canal de contato.
2. Sistema abre uma solicitação com prazo e responsável.
3. Agente autorizado localiza dados por busca protegida.
4. Ação proposta é revisada quando impactar obrigação legal.
5. Entrega ou correção usa canal seguro.
6. Conclusão é auditada sem guardar cópia desnecessária dos dados entregues.

## Integração FNRH

Cada estabelecimento optante fornece sua própria credencial. O sistema:

- cifra a credencial usando KMS;
- transforma o modelo local para a versão ativa do adaptador;
- envia com timeout e correlação;
- classifica retorno em sucesso, falha permanente ou tentativa posterior;
- não perde nem duplica a estadia local;
- disponibiliza reconciliação por período;
- nunca revela a credencial à interface após o cadastro.

O projeto municipal não deve presumir acesso centralizado aos registros de todos
os estabelecimentos na FNRH. A integração deve ser validada com o Ministério do
Turismo e com cada estabelecimento.
