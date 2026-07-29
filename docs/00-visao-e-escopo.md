# Visão, atores e escopo

## Problema

Comerciantes, poder público e moradores não dispõem de uma visão confiável e
antecipada sobre quantas pessoas estarão em Cumuruxatiba, por quanto tempo e com
quais necessidades. Reservas estão distribuídas entre pousadas, campings, casas
de temporada e hospedagens informais.

## Objetivo

Produzir inteligência turística útil para planejamento de estoque, equipes,
eventos, mobilidade, saneamento e serviços públicos sem construir uma base
comercial de pessoas.

## Atores

| Ator | Necessidade | Acesso |
|---|---|---|
| Hóspede | preencher rapidamente e revisar seus dados | sua estadia/grupo |
| Responsável pelo grupo | cadastrar acompanhantes | seu grupo |
| Operador da hospedagem | criar e corrigir estadias | seu estabelecimento |
| Responsável da hospedagem | gerir operadores e integração | seu estabelecimento |
| Gestor municipal | administrar política e qualidade | dados operacionais autorizados |
| Encarregado de dados | revisar perguntas, incidentes e direitos | módulo de privacidade |
| Analista | acompanhar cobertura e agregados internos | dados pseudonimizados |
| Público/comércio | planejar demanda | dashboard agregado |
| Auditor | verificar ações administrativas | trilha somente leitura |

## Duas jornadas separadas

### Registro operacional

Serve para criar uma estadia, contar pessoas e, quando aplicável, transmitir os
dados legais à FNRH. Coleta somente o necessário à finalidade declarada.

### Pesquisa turística

Serve para preferências e expectativas. É apresentada separadamente, deixa claro
o que é opcional e não impede o check-in quando recusada.

## Escopo do MVP

- cadastro e aprovação de hospedagens/anfitriões;
- criação de estadia e grupo;
- link, QR Code e código curto;
- autopreenchimento e preenchimento assistido;
- check-in, alteração de saída e check-out;
- questionários editáveis, versionados e condicionais;
- consentimentos separados quando a base escolhida os exigir;
- agregados diários e dashboard público;
- idempotência, auditoria e reconciliação;
- integração FNRH como piloto controlado;
- fluxo de solicitação de acesso/correção/eliminação;
- cobertura do sistema e qualidade dos dados.

## Fora do MVP

- aplicativo nativo;
- marketplace de serviços;
- marketing direto;
- pagamento, reserva ou cobrança;
- reconhecimento facial, biometria ou rastreamento de localização;
- venda ou compartilhamento de leads;
- painel por estabelecimento;
- exportação pública de microdados;
- machine learning complexo;
- integração direta com plataformas de aluguel sem contrato e base legal.

## Métricas de sucesso

| Métrica | Meta inicial |
|---|---|
| Hospedagens participantes | pelo menos 60% da capacidade conhecida no piloto |
| Registros duplicados | menos de 0,5% após reconciliação |
| Formulário básico concluído | 90% em até 3 minutos |
| Disponibilidade mensal | 99,5% no piloto |
| Atualização do painel | até 15 minutos; diária para reconciliação |
| Cobertura informada | exibida em todo indicador de previsão |
| Incidentes com exposição pessoal | zero |

## Regras de produto

- Uma pessoa pertence a uma estadia; uma estadia pertence a uma acomodação.
- Um grupo pode ter um responsável e vários integrantes.
- Saída prevista alimenta previsão; saída real alimenta histórico.
- Cancelamento e `no-show` nunca contam como presença.
- Uma pergunta publicada não muda; uma nova edição gera nova versão.
- Uma resposta livre nunca vira dimensão pública automaticamente.
- Toda métrica pública carrega período de referência, atualização, cobertura e
  regra de privacidade.
