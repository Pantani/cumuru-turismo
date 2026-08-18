# Operação, resiliência e implantação

## Ambientes

```text
local -> test -> staging -> production
```

Cada ambiente possui banco, chaves, provedor OIDC e credenciais externos
independentes. Dados pessoais de produção não são copiados para desenvolvimento.

## Topologia de produção inicial

- CDN/WAF para front e endpoints públicos;
- balanceador gerenciado;
- duas réplicas pequenas da API em zonas distintas;
- duas réplicas do worker;
- PostgreSQL gerenciado com failover e PITR;
- KMS/secret manager;
- coletor OpenTelemetry;
- armazenamento de backup e artefatos.

Para piloto pequeno, API e worker podem começar com uma réplica cada, desde que a
plataforma reinicie automaticamente e o banco tenha backup comprovado.

## Objetivos iniciais

| Objetivo | Valor |
|---|---|
| Disponibilidade mensal da API | 99,5% |
| p95 de leitura autenticada | menor que 500 ms |
| p95 de escrita local | menor que 800 ms |
| Atualização do dashboard | até 15 min |
| RPO | 15 min |
| RTO | 4 h |
| Recuperação de job externo | até 24 h |

Integração FNRH não faz parte do tempo de resposta da criação local.

## Jobs

Tabela de jobs com:

- tipo;
- payload mínimo;
- deduplication key;
- tentativas;
- `available_at`;
- lease;
- último erro sanitizado;
- estado;
- dead letter.

Política de retry:

- timeout, conexão e `429/5xx`: exponencial com jitter;
- `400/401/403/422`: falha permanente ou ação humana;
- máximo de tentativas por tipo;
- circuit breaker em memória por instância;
- reconciliação periódica corrige perda de sinal.

No questionário, o cleanup idempotente de texto livre remove ciphertext, nonce e
versão da chave depois de `erase_after`, limitado a 24 horas em `local|test`.
O job não copia conteúdo para log, audit ou outbox. Publicação e submissão de
pergunta livre falham fechadas quando cifra AES-GCM-256 ou cleanup não estão
habilitados. Esse prazo técnico não substitui política jurídica de retenção.

## Outbox

Todo efeito externo nasce em `platform.outbox_event` na mesma transação do dado
de domínio. Publicação usa `event_id` como chave estável. O consumidor registra
processamento antes de aplicar efeitos não repetíveis.

## Deploy

1. build reproduzível e SBOM;
2. testes e scanners;
3. backup/restore point;
4. migrações compatíveis para frente;
5. deploy da API/worker;
6. smoke tests;
7. publicação do front;
8. observação;
9. remoção de compatibilidade antiga apenas em release posterior.

Use expand/contract em mudanças de schema:

1. adicionar coluna/tabela;
2. escrever nos dois formatos quando necessário;
3. backfill idempotente;
4. trocar leitura;
5. remover versão antiga em outro deploy.

## Observabilidade

### Métricas

- taxa, erro e latência por rota;
- conexões e queries lentas;
- tamanho e idade das filas;
- falhas e latência FNRH;
- idempotency conflicts;
- rascunhos enviados e rejeitados;
- atraso dos agregados;
- supressões do dashboard;
- cobertura e qualidade.

### Alertas

- erro 5xx acima do limite;
- nenhum dado agregado novo;
- fila mais antiga que o SLO;
- backup ou restore test falhou;
- aumento de 401/403;
- acesso de privacidade fora do padrão;
- falhas permanentes FNRH;
- armazenamento ou conexões perto do limite.

## Degradação controlada

- FNRH indisponível: registrar localmente e marcar sincronização pendente.
- provedor de mensagem indisponível: exibir link/QR e tentar depois.
- agregador indisponível: servir última versão pública válida.
- OIDC indisponível: público continua; novos logins pausam; sessões válidas
  seguem política definida.
- banco indisponível: formulário preserva rascunho local e informa pendência,
  sem prometer conclusão.

## Contingência em papel

Se houver obrigação legal, deve existir procedimento acessível para falta de
internet ou dispositivo. O papel:

- usa o menor conjunto de campos;
- possui cadeia de custódia;
- é digitado por pessoa autorizada;
- é destruído de modo seguro após confirmação e prazo aplicável;
- não vira planilha paralela permanente.

## Runbooks obrigatórios

- API fora do ar;
- banco indisponível;
- restore de backup;
- fila travada;
- credencial FNRH revogada;
- agregados incorretos;
- suspeita de vazamento;
- exclusão/anonimização;
- rotação de chaves;
- rollback de release.

## Capacidade

Planejar pico de Réveillon/Carnaval. Teste inicial:

- 100 requisições/s de leitura pública com cache frio;
- 20 submissões/s;
- grupos de até 30 integrantes;
- questionários de até 100 perguntas;
- 100 mil visitantes/ano;
- recomputação de 3 anos em janela operacional definida.

Os números são hipótese de teste e devem ser ajustados após o censo de
hospedagens.
