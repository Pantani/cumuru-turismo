# Decisões técnicas

## ADR-001 — Go para API e workers

**Status:** aceito.

Go produz binário simples, tem boa biblioteca HTTP, concorrência previsível e
baixo consumo operacional. A linha 1.26 é a atual na data deste blueprint; usar
sempre a versão patch suportada mais recente.

Evitar framework web amplo. Começar com `net/http`; adicionar biblioteca somente
quando reduzir complexidade comprovada.

## ADR-002 — React em vez de uma alternativa menor

**Status:** aceito.

Preact ou Lit produziriam bundle menor, mas este produto terá:

- formulários dinâmicos extensos;
- regras condicionais;
- modo offline;
- múltiplos perfis administrativos;
- gráficos e acessibilidade;
- necessidade de manutenção por equipes variadas.

React oferece maior ecossistema e disponibilidade de profissionais. O custo de
bundle será controlado com Vite, divisão por rota e carregamento tardio dos
gráficos. Não usar Next.js no MVP porque SEO dinâmico e SSR não justificam outro
runtime no servidor.

## ADR-003 — Vite para build

**Status:** aceito.

O front é estático e consome API Go. Vite atende TypeScript, desenvolvimento
rápido e build de produção sem trazer um backend Node. Create React App está
descontinuado.

## ADR-004 — PostgreSQL como banco e fila inicial

**Status:** aceito.

Jobs e outbox precisam ser atômicos com dados de negócio. PostgreSQL com
`FOR UPDATE SKIP LOCKED`, índices parciais e leases atende a escala inicial e
elimina Redis/Kafka.

Reavaliar se:

- idade p95 da fila exceder SLO mesmo após tuning e réplicas;
- banco transacional sofrer contenção mensurável;
- houver necessidade real de fan-out em grande escala.

## ADR-005 — Monólito modular

**Status:** aceito.

Domínios separados em pacotes e esquemas, mas uma aplicação implantável. Extrair
serviço somente quando houver demanda operacional, equipe responsável e
contrato estável.

## ADR-006 — OIDC externo

**Status:** aceito.

Não armazenar senha. A aplicação valida issuer, audience, assinatura, expiração e
escopos. O provedor definitivo depende de contratação e requisitos do Município.

## ADR-007 — OpenAPI primeiro

**Status:** aceito.

O contrato gera tipos do cliente, testes e documentação. Mudanças incompatíveis
seguem versionamento. Código gerado não é editado.

## ADR-008 — FNRH por adaptador

**Status:** aceito.

A documentação do Ministério alerta que versões da API podem mudar e cada meio
de hospedagem possui sua própria chave. O domínio local não importa DTOs da FNRH.
O adaptador traduz, versiona e reconcilia.

## ADR-009 — Publicação por tabela liberada

**Status:** aceito.

Dashboard público lê apenas `public_data`. Views diretas sobre microdados não são
suficientes porque filtros, joins e mudanças podem reintroduzir risco. Um job
materializa e publica uma versão depois das regras de supressão.

## ADR-010 — Sem nome no domínio analítico

**Status:** aceito.

Nome pode existir no cofre de identidade quando necessário à operação/FNRH, mas
não é atributo de visitante analítico. O worker recebe somente ID pseudônimo e
atributos generalizados.

## Referências técnicas atuais

- [Go — histórico e política de versões](https://go.dev/doc/devel/release)
- [Go 1.26](https://go.dev/doc/go1.26)
- [React — versões](https://react.dev/versions)
- [React — descontinuação do Create React App](https://react.dev/blog/2025/02/14/sunsetting-create-react-app)
- [Vite — guia](https://vite.dev/guide/)
- [PostgreSQL — documentação](https://www.postgresql.org/docs/)

Verificação: 27 de julho de 2026. Fixar versões exatas no lockfile e no
toolchain da primeira fase.
