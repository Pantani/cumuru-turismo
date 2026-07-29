# Roadmap e critérios de aceite

## Fase 0 — Governança e descoberta

Entregas:

- patrocinador e controlador definidos;
- mapa de hospedagens e capacidade;
- fluxo atual da FNRH;
- parecer jurídico;
- inventário de dados;
- RIPD inicial;
- política de publicação;
- seleção do provedor OIDC e infraestrutura.

Gate: sem essas decisões, apenas protótipo com dados fictícios.

## Fase 1 — Fundação técnica

Entregas:

- monorepo;
- API Go, worker e React;
- configuração por ambiente;
- PostgreSQL e migrações;
- OIDC;
- OpenAPI e cliente gerado;
- logs, traces, métricas e health checks;
- CI, SBOM e scanners.

Aceite:

- ambiente sobe do zero com um comando documentado;
- migrações aplicam e revertem conforme política;
- nenhuma rota autenticada aceita token inválido;
- teste de isolamento por organização passa.

## Fase 2 — Hospedagens e estadias

Entregas:

- hospedagens, usuários e papéis;
- estadia, grupo e visitantes;
- convite/QR Code;
- rascunho offline;
- check-in, check-out, cancelamento e no-show;
- idempotência e concorrência otimista;
- auditoria.

Aceite:

- repetir qualquer submissão não duplica visitante;
- mesmo convite com conteúdo diferente gera conflito;
- operador de A não acessa B;
- alteração de saída recalcula dias;
- cancelado não aparece na presença.

## Fase 3 — Questionário

Entregas:

- editor;
- revisão de privacidade;
- versões imutáveis;
- perguntas condicionais;
- resposta separada da estadia;
- consentimentos versionados.

Aceite:

- pergunta publicada não pode ser alterada;
- nova versão não muda gráficos antigos;
- pergunta sensível não é publicável sem aprovação;
- recusa da pesquisa não impede check-in;
- texto livre não entra em dimensão pública.

## Fase 4 — Analytics e dashboard

Entregas:

- presença diária;
- cobertura;
- cubos permitidos;
- supressão e arredondamento;
- painel público e metodologia;
- painel interno de qualidade.

Aceite:

- papel público só lê esquema `public_data`;
- nenhum payload contém IDs internos;
- células pequenas e complementares são suprimidas;
- previsão diferencia observado e estimado;
- painel mostra atualização e cobertura;
- reconciliação produz o mesmo resultado duas vezes.

## Fase 5 — FNRH piloto

Entregas:

- cofre de credenciais;
- adaptador para versão vigente;
- mapeamento de domínios;
- retry/dead letter;
- reconciliação;
- telas de status para a hospedagem.

Aceite:

- credencial nunca aparece em log ou leitura posterior;
- timeout não perde registro local;
- reenvio não cria efeito duplicado;
- falha de validação é acionável;
- contrato validado em homologação oficial;
- cada estabelecimento usa sua própria credencial.

## Fase 6 — Piloto controlado

Escopo sugerido:

- 5 a 10 hospedagens de perfis diferentes;
- dados reais somente após gates legais;
- 30 a 60 dias;
- suporte e canal do titular ativos;
- reunião semanal de qualidade.

Critérios para ampliar:

- adesão e cobertura mínimas;
- erro de duplicidade abaixo da meta;
- previsão com erro acompanhado;
- nenhum incidente crítico;
- tempo de preenchimento aceitável;
- restauração e resposta a incidente simuladas;
- parecer do comitê de governança.

## Backlog posterior

- alertas de demanda para comerciantes;
- API aberta de agregados;
- integração com calendários de eventos e clima;
- importação por PMS adicionais;
- pesquisa pós-visita;
- modelo preditivo avançado;
- acessibilidade em idiomas adicionais;
- portal de transparência e relatórios periódicos.

## Definition of Done

Uma história está pronta quando:

- requisito e ameaça foram considerados;
- contrato e migração estão atualizados;
- autorização negativa está testada;
- repetição e concorrência estão testadas;
- logs não vazam conteúdo;
- acessibilidade foi verificada;
- teste automatizado passa;
- documentação operacional existe;
- resultado foi demonstrado com dados fictícios;
- itens não verificados estão explícitos.
