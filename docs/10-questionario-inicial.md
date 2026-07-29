# Questionário inicial proposto

## Regra de separação

O formulário possui três blocos visualmente distintos:

1. estadia;
2. integrantes;
3. pesquisa turística opcional.

## Bloco 1 — Estadia

| Campo | Obrigatório | Motivo |
|---|---:|---|
| acomodação | sim | vínculo e cobertura |
| data prevista de entrada | sim | previsão |
| data prevista de saída | sim | previsão |
| quantidade prevista | sim | capacidade |
| referência da reserva | não | reconciliação |
| tipo de grupo | não | estatística |

## Bloco 2 — Integrantes

| Campo | Obrigatório | Observação |
|---|---:|---|
| nome | condicional | somente operação/FNRH, não analytics |
| documento | condicional | apenas se exigido pela finalidade/FNRH |
| faixa etária | sim | preferir a idade/data exata |
| país de residência | sim | perfil geral |
| UF de origem | se Brasil | agregação |
| município de origem | se Brasil | generalizado na publicação |
| responsável ou acompanhante | sim | regra do grupo |
| contato | somente responsável | envio/revisão, com retenção curta |

O sistema não deve chamar nome ou documento de “dados estatísticos”. Esses dados
servem à operação legal quando aplicável e ficam separados.

## Bloco 3 — Pesquisa turística

Perguntas estruturadas e opcionais:

1. É sua primeira visita a Cumuruxatiba?
2. Qual é o principal motivo desta viagem?
3. Como conheceu Cumuruxatiba?
4. Qual foi o principal meio de transporte?
5. Com que antecedência planejou a viagem?
6. Quais atividades pretende realizar?
7. Que experiências procura?
8. Que tipos de alimentação procura?
9. Pretende contratar passeios?
10. Qual faixa aproximada de gasto diário por pessoa?
11. Em que períodos costuma consumir no comércio?
12. Prefere tranquilidade, música ao vivo, cultura ou festa?
13. Quais estilos musicais gostaria de ouvir?
14. Tem interesse em artesanato e produtos locais?
15. Que serviço considera importante encontrar?
16. O que espera de Cumuruxatiba?

## Opções sugeridas

### Motivo

```text
lazer e descanso
visita a amigos ou família
evento
trabalho
natureza e ecoturismo
cultura
outro
prefiro não responder
```

### Atividades

```text
praia
passeio de barco
trilhas e natureza
gastronomia
artesanato
programação cultural
música ao vivo
vida noturna
descanso
atividades com crianças
```

### Ambiente

Permitir múltipla escolha:

```text
silêncio e descanso
ambiente familiar
música ambiente
música ao vivo
eventos culturais
festas
```

Evitar a formulação binária “silêncio ou festa”, que força perfis incompatíveis
com o comportamento real.

### Música

```text
MPB
forró
samba
reggae
axé
rock
música eletrônica
jazz e instrumental
música regional
outro
não procuro programação musical
```

### Gasto diário

As faixas devem ser revisadas periodicamente sem reescrever respostas antigas.
Cada revisão cria opções versionadas.

## Texto livre

“O que espera de Cumuruxatiba?” é útil qualitativamente, mas não deve ser
publicado automaticamente. Exibir:

> Não informe nome, contato, saúde, religião, opinião política ou dados de outra
> pessoa.

Análise pública inicial deve usar somente tags atribuídas por processo
controlado. O texto original permanece restrito e segue retenção curta.

## Perguntas a evitar

- CPF sem necessidade operacional/legal;
- endereço completo de origem;
- renda exata;
- religião;
- opinião política;
- saúde ou deficiência em pergunta de mercado;
- biometria ou fotografia;
- localização contínua;
- identificação de crianças para preferências;
- consentimento único para finalidades diferentes;
- pergunta livre obrigatória.

## Metadados de cada pergunta

Exemplo:

```yaml
stable_key: preferred_environment
purpose: tourism_demand_planning
classification: personal
required: false
answer_type: multiple_choice
public_aggregation_allowed: true
minimum_public_cell: 10
retention_policy: survey_standard_v1
privacy_review: approved
```
