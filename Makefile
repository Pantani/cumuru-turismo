HARNESS := .agents/skills/cumuru-bootstrap/scripts/harness.sh
HARNESS_TEST := .agents/skills/cumuru-bootstrap/scripts/test-harness.sh
TOOLS_BIN := $(CURDIR)/.tools/bin
STATICCHECK := $(TOOLS_BIN)/staticcheck
GOVULNCHECK := $(TOOLS_BIN)/govulncheck
CYCLONEDX_GOMOD := $(TOOLS_BIN)/cyclonedx-gomod
GOCYCLO := $(TOOLS_BIN)/gocyclo
GOCOGNIT := $(TOOLS_BIN)/gocognit
CUMURU_IMAGE_TAG ?= 0.2.0
CUMURU_API_IMAGE := cumuru-api:$(CUMURU_IMAGE_TAG)
CUMURU_WEB_IMAGE := cumuru-web:$(CUMURU_IMAGE_TAG)
TRIVY_IMAGE := aquasec/trivy:0.69.3@sha256:bcc376de8d77cfe086a917230e818dc9f8528e3c852f7b1aff648949b6258d1c
GITLEAKS_IMAGE := zricethezav/gitleaks:v8.30.1@sha256:c00b6bd0aeb3071cbcb79009cb16a60dd9e0a7c60e2be9ab65d25e6bc8abbb7f
WITH_BUILD_METADATA := deploy/scripts/with-build-metadata.sh
LOCAL_COMPOSE := docker compose -f compose.yaml -f compose.local.yaml
DEV_COMPOSE := docker compose -p cumuru-dev -f compose.yaml -f compose.local.yaml -f compose.dev.yaml
DOCKER_DEV_SERVICES ?=
IMAGE_ARTIFACTS := deploy/scripts/image-artifacts.sh
MATERIALIZE_PINNED_IMAGE := deploy/scripts/materialize-pinned-image.sh
RTK ?= rtk
AI_PATHS ?= .
AI_GO_PACKAGES ?= ./...
AI_FRONTEND_SPEC ?=
AI_QUERY ?=
DOCKER_SERVICES ?=
DOCKER_LOG_TAIL ?= 200

export CUMURU_IMAGE_TAG
export RTK AI_PATHS AI_GO_PACKAGES AI_FRONTEND_SPEC AI_QUERY
export DOCKER_SERVICES DOCKER_LOG_TAIL

.DEFAULT_GOAL := help

.PHONY: \
	help setup clean dev dev-web check security infra-validation tidy vet \
	test-backend test-backend-short test-backend-race \
	test-frontend test-frontend-unit test-short test-unit test-all test-race \
	docker-up docker-down docker-status docker-logs docker-restart \
	ai-help ai-status ai-files ai-diff-stat ai-diff ai-grep ai-gain \
	ai-test-backend-short ai-test-backend ai-test-frontend-unit \
	ai-test-frontend-file ai-lint-backend ai-lint-frontend \
	ai-build-frontend ai-check \
	harness-validate harness-test harness-status harness-phase harness-prompt \
	harness-dry-run harness-snapshot install tools openapi-lint generate \
	generate-web generate-sqlc generated-check migration-test build test \
	local-restore-drill \
	phase2-integration phase2-proxy-test phase2-full-stack typecheck complexity \
	phase3-integration phase3-proxy-test \
	phase4-integration phase4-proxy-test phase4-full-stack phase4-benchmark \
	local-demo-test local-demo-e2e phase4-remediation \
	docker-dev docker-dev-down docker-dev-logs docker-dev-status seed \
	post-task-quality lint-shell lint lint-fix images sbom image-sbom scanner-images scan image-scan compose-config up down migrate-up \
	migrate-down-local smoke ci

help: ## Lista os targets públicos disponíveis
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_-]+:.*## / {printf "  %-28s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

harness-validate: ## Valida a estrutura e o contrato do harness
	@bash "$(HARNESS)" validate

harness-test: ## Executa os testes do harness
	@bash "$(HARNESS_TEST)"

harness-status: ## Mostra o estado das fases e gates do harness
	@bash "$(HARNESS)" status

harness-phase: ## Executa a fase informada em PHASE
	@test -n "$(PHASE)" || (echo "PHASE is required" >&2; exit 2)
	@bash "$(HARNESS)" phase "$(PHASE)"

harness-prompt: ## Extrai o prompt da fase informada em PHASE
	@test -n "$(PHASE)" || (echo "PHASE is required" >&2; exit 2)
	@bash "$(HARNESS)" prompt "$(PHASE)"

harness-dry-run: ## Simula a fase informada em PHASE sem alterar a aplicação
	@test -n "$(PHASE)" || (echo "PHASE is required" >&2; exit 2)
	@bash "$(HARNESS)" dry-run "$(PHASE)"

harness-snapshot: ## Preserva a tentativa da fase PHASE antes de reexecução ampla
	@test -n "$(PHASE)" || (echo "PHASE is required" >&2; exit 2)
	@bash "$(HARNESS)" snapshot "$(PHASE)" "$(ATTEMPT)"

install: ## Instala dependências npm e Go; usa rede quando o cache não basta
	npm ci
	go -C apps/api mod download

tools: ## Instala ferramentas Go fixadas em .tools/bin; usa rede
	@mkdir -p "$(TOOLS_BIN)"
	GOBIN="$(TOOLS_BIN)" go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
	GOBIN="$(TOOLS_BIN)" go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
	GOBIN="$(TOOLS_BIN)" go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0
	GOBIN="$(TOOLS_BIN)" go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
	GOBIN="$(TOOLS_BIN)" go install github.com/uudashr/gocognit/cmd/gocognit@v1.2.1

setup: ## Instala dependências e ferramentas, sem gerar ou alterar contratos
	@$(MAKE) --no-print-directory install
	@$(MAKE) --no-print-directory tools

openapi-lint: ## Valida o contrato OpenAPI
	npm exec -- redocly lint contracts/openapi.yaml

generate: generate-web generate-sqlc ## Regenera cliente web e queries sqlc

generate-web: ## Regenera o cliente TypeScript a partir do OpenAPI
	npm run generate:web

generate-sqlc: ## Regenera código Go via sqlc
	cd apps/api && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate

generated-check: ## Verifica reprodutibilidade dos arquivos gerados
	@bash deploy/scripts/check-generated.sh

migration-test: ## Testa migrations e grants em PostgreSQL real via Docker
	@bash deploy/scripts/test-migrations.sh

local-restore-drill: ## Prova dump/restore sintético em PostgreSQL isolado
	@bash deploy/scripts/test-local-restore.sh

phase2-integration: ## Executa a integração PostgreSQL da Fase 2 via Docker
	@bash deploy/scripts/test-phase2-integration.sh

phase2-proxy-test: ## Testa o hardening dos proxies da Fase 2 via Docker
	@bash deploy/scripts/test-proxy-hardening.sh

phase2-full-stack: ## Testa a stack completa da Fase 2 via Docker
	@bash deploy/scripts/test-phase2-full-stack.sh

phase3-integration: ## Executa a integração PostgreSQL da Fase 3 via Docker
	@bash deploy/scripts/test-phase3-integration.sh

phase3-proxy-test: ## Testa o proxy da Fase 3 via Docker
	@bash deploy/scripts/test-phase3-proxy.sh

phase4-integration: ## Executa a integração PostgreSQL da Fase 4 via Docker
	@bash deploy/scripts/test-phase4-integration.sh

phase4-proxy-test: ## Testa o proxy da Fase 4 via Docker
	@bash deploy/scripts/test-phase4-proxy.sh

phase4-full-stack: ## Testa a stack completa da Fase 4 via Docker
	@bash deploy/scripts/test-phase4-full-stack.sh

phase4-benchmark: phase4-full-stack ## Executa o benchmark protegido da Fase 4 via Docker
	@bash deploy/scripts/benchmark-phase4-recompute.sh

local-demo-test: ## Valida seed local em banco novo, repetição e preservação
	@bash deploy/scripts/test-local-demo.sh

local-demo-e2e: ## Executa a jornada local completa em Chromium e stack efêmera
	@bash deploy/scripts/test-local-demo-e2e.sh

phase4-remediation: ## Executa o build reproduzível de remediação do runtime local
	@$(MAKE) --no-print-directory generated-check
	@$(MAKE) --no-print-directory local-demo-test
	@$(MAKE) --no-print-directory phase4-full-stack
	@$(MAKE) --no-print-directory local-demo-e2e

build: ## Compila API, worker e web
	go -C apps/api build ./cmd/api ./cmd/worker ./cmd/localdemo
	npm --workspace @cumuru/web run build

test: ## Executa as suítes canônicas de Go e web
	go -C apps/api test ./...
	npm --workspace @cumuru/web run test

test-backend: ## Executa todos os testes Go
	go -C apps/api test ./...

test-backend-short: ## Executa os testes Go rápidos sem cache
	go -C apps/api test -short -count=1 ./...

test-backend-race: ## Executa todos os testes Go com race detector e sem cache
	go -C apps/api test -race -count=1 ./...

test-frontend: ## Executa a suíte Vitest do web
	npm --workspace @cumuru/web run test

test-frontend-unit: ## Alias explícito da suíte Vitest atual
	@$(MAKE) --no-print-directory test-frontend

test-short: ## Alias do bundle rápido test-unit
	@$(MAKE) --no-print-directory test-unit

test-unit: ## Executa Go short e a suíte Vitest, sem Docker
	@$(MAKE) --no-print-directory test-backend-short
	@$(MAKE) --no-print-directory test-frontend-unit

test-all: ## Alias da suíte canônica completa make test
	@$(MAKE) --no-print-directory test

test-race: ## Executa Go com race detector e a suíte Vitest
	@$(MAKE) --no-print-directory test-backend-race
	@$(MAKE) --no-print-directory test-frontend-unit

vet: ## Executa go vet no backend
	go -C apps/api vet ./...

tidy: ## Atualiza go.mod/go.sum com go mod tidy; target mutante
	go -C apps/api mod tidy

typecheck: ## Executa o typecheck estrito do web
	npm --workspace @cumuru/web run typecheck

complexity: ## Verifica complexidade ciclomática 5 e cognitiva 8 em Go e web
	@bash deploy/scripts/test-complexity.sh "$(GOCYCLO)" "$(GOCOGNIT)"

post-task-quality: ## Gate obrigatório pós-tarefa: complexity, lint e marcador PASS
	@$(MAKE) --no-print-directory complexity
	@$(MAKE) --no-print-directory lint
	@echo "POST_TASK_QUALITY=PASS"

lint-shell: ## Valida a sintaxe de todos os scripts shell próprios
	@set -eu; \
	files="$$(mktemp "$${TMPDIR:-/tmp}/cumuru-shell-lint.XXXXXX")"; \
	trap 'rm -f -- "$$files"' 0 1 2 3 15; \
	find . -type f -name '*.sh' \
		-not -path './.git/*' \
		-not -path './node_modules/*' \
		-not -path './_workspace/*' \
		-not -path './artifacts/*' \
		-not -path './apps/web/dist/*' \
		-print >"$$files"; \
	count="$$(wc -l <"$$files" | tr -d '[:space:]')"; \
	test "$$count" -gt 0; \
	while IFS= read -r file; do \
		bash -n "$$file"; \
	done <"$$files"; \
		echo "SHELL_SYNTAX=PASS files=$$count"

lint: lint-shell ## Executa shell, go vet, Staticcheck e lint do web
	go -C apps/api vet ./...
	cd apps/api && "$(STATICCHECK)" ./...
	npm --workspace @cumuru/web run lint

lint-fix: ## Aplica correções automáticas seguras de lint e formatação no monorepo
	find apps/api -type f -name '*.go' -exec gofmt -w {} +
	npm --workspace @cumuru/web exec -- oxlint --config .oxlintrc.json --deny-warnings --fix .
	@$(MAKE) --no-print-directory lint

check: ## Executa o gate local sequencial, sem Docker ou scanners
	@$(MAKE) --no-print-directory openapi-lint
	@$(MAKE) --no-print-directory generated-check
	@$(MAKE) --no-print-directory test-all
	@$(MAKE) --no-print-directory typecheck
	@$(MAKE) --no-print-directory post-task-quality
	@$(MAKE) --no-print-directory build

security: ## Executa os scanners canônicos; requer rede e Docker
	@$(MAKE) --no-print-directory scan

infra-validation: ## Valida a infraestrutura sem aplicar ou fazer deploy
	@bash deploy/scripts/validate-infra.sh

clean: ## Remove somente apps/web/dist e artifacts/coverage
	rm -rf -- apps/web/dist artifacts/coverage

ai-help: ## Lista os atalhos seguros para agentes via RTK
	@echo "Atalhos AI via $$RTK:"
	@echo "  ai-status | ai-files | ai-diff-stat | ai-diff"
	@echo "  ai-grep AI_QUERY=regex [AI_PATHS='path ...']"
	@echo "  ai-test-backend-short | ai-test-backend"
	@echo "  ai-test-frontend-unit"
	@echo "  ai-test-frontend-file AI_FRONTEND_SPEC=src/arquivo.test.ts"
	@echo "  ai-lint-backend | ai-lint-frontend | ai-build-frontend | ai-check"
	@echo "  ai-gain"
	@echo "Observação: git diff não inclui conteúdo untracked; use ai-status junto."

ai-status: ## AI: mostra o status Git curto via RTK
	@command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	"$$RTK" git status --short

ai-files: ## AI: mostra status e nomes alterados nos paths validados
	@set -eu; \
	command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	set --; \
	for path in $$AI_PATHS; do \
		case "$$path" in \
			/*|..|../*|*/../*|*/..) echo "invalid AI_PATHS entry: $$path" >&2; exit 2 ;; \
			*[!A-Za-z0-9_./-]*) echo "invalid AI_PATHS entry: $$path" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$path"; \
	done; \
	test "$$#" -gt 0 || { echo "AI_PATHS is required" >&2; exit 2; }; \
	"$$RTK" git status --short -- "$$@"; \
	"$$RTK" git diff --name-status -- "$$@"; \
	"$$RTK" git diff --cached --name-status -- "$$@"

ai-diff-stat: ## AI: mostra diffstat tracked nos AI_PATHS validados
	@set -eu; \
	command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	set --; \
	for path in $$AI_PATHS; do \
		case "$$path" in \
			/*|..|../*|*/../*|*/..) echo "invalid AI_PATHS entry: $$path" >&2; exit 2 ;; \
			*[!A-Za-z0-9_./-]*) echo "invalid AI_PATHS entry: $$path" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$path"; \
	done; \
	test "$$#" -gt 0 || { echo "AI_PATHS is required" >&2; exit 2; }; \
	"$$RTK" git diff --stat -- "$$@"; \
	"$$RTK" git diff --cached --stat -- "$$@"

ai-diff: ## AI: mostra o diff tracked nos AI_PATHS validados
	@set -eu; \
	command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	set --; \
	for path in $$AI_PATHS; do \
		case "$$path" in \
			/*|..|../*|*/../*|*/..) echo "invalid AI_PATHS entry: $$path" >&2; exit 2 ;; \
			*[!A-Za-z0-9_./-]*) echo "invalid AI_PATHS entry: $$path" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$path"; \
	done; \
	test "$$#" -gt 0 || { echo "AI_PATHS is required" >&2; exit 2; }; \
	"$$RTK" git diff -- "$$@"; \
	"$$RTK" git diff --cached -- "$$@"

ai-grep: ## AI: busca AI_QUERY via RTK nos AI_PATHS validados
	@set -eu; \
	command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	test -n "$$AI_QUERY" || { echo "AI_QUERY is required" >&2; exit 2; }; \
	set --; \
	for path in $$AI_PATHS; do \
		case "$$path" in \
			/*|..|../*|*/../*|*/..) echo "invalid AI_PATHS entry: $$path" >&2; exit 2 ;; \
			*[!A-Za-z0-9_./-]*) echo "invalid AI_PATHS entry: $$path" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$path"; \
	done; \
	test "$$#" -gt 0 || { echo "AI_PATHS is required" >&2; exit 2; }; \
	"$$RTK" rg -n -- "$$AI_QUERY" "$$@"

ai-gain: ## AI: mostra a economia recente e histórica do RTK
	@command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	"$$RTK" gain; \
	"$$RTK" gain --history

ai-test-backend-short: ## AI: executa pacotes Go short validados via RTK
	@set -eu; \
	command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	set --; \
	for package in $$AI_GO_PACKAGES; do \
		case "$$package" in \
			./..|./../*|*/../*|*/..) echo "invalid AI_GO_PACKAGES entry: $$package" >&2; exit 2 ;; \
			./*) ;; \
			*) echo "invalid AI_GO_PACKAGES entry: $$package" >&2; exit 2 ;; \
		esac; \
		case "$$package" in \
			*[!A-Za-z0-9_./-]*) echo "invalid AI_GO_PACKAGES entry: $$package" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$package"; \
	done; \
	test "$$#" -gt 0 || { echo "AI_GO_PACKAGES is required" >&2; exit 2; }; \
	"$$RTK" go -C apps/api test -short -count=1 "$$@"

ai-test-backend: ## AI: executa pacotes Go validados via RTK
	@set -eu; \
	command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	set --; \
	for package in $$AI_GO_PACKAGES; do \
		case "$$package" in \
			./..|./../*|*/../*|*/..) echo "invalid AI_GO_PACKAGES entry: $$package" >&2; exit 2 ;; \
			./*) ;; \
			*) echo "invalid AI_GO_PACKAGES entry: $$package" >&2; exit 2 ;; \
		esac; \
		case "$$package" in \
			*[!A-Za-z0-9_./-]*) echo "invalid AI_GO_PACKAGES entry: $$package" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$package"; \
	done; \
	test "$$#" -gt 0 || { echo "AI_GO_PACKAGES is required" >&2; exit 2; }; \
	"$$RTK" go -C apps/api test -count=1 "$$@"

ai-test-frontend-unit: ## AI: executa a suíte Vitest via RTK
	@command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	"$$RTK" npm --workspace @cumuru/web run test

ai-test-frontend-file: ## AI: executa AI_FRONTEND_SPEC validado via Vitest
	@set -eu; \
	command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	test -n "$$AI_FRONTEND_SPEC" || { echo "AI_FRONTEND_SPEC is required" >&2; exit 2; }; \
	case "$$AI_FRONTEND_SPEC" in \
		-*) echo "invalid AI_FRONTEND_SPEC: $$AI_FRONTEND_SPEC" >&2; exit 2 ;; \
		/*|..|../*|*/../*|*/..) echo "invalid AI_FRONTEND_SPEC: $$AI_FRONTEND_SPEC" >&2; exit 2 ;; \
		*[!A-Za-z0-9_./-]*) echo "invalid AI_FRONTEND_SPEC: $$AI_FRONTEND_SPEC" >&2; exit 2 ;; \
	esac; \
	"$$RTK" npm --workspace @cumuru/web run test -- "$$AI_FRONTEND_SPEC"

ai-lint-backend: ## AI: executa go vet e Staticcheck via RTK
	@command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	"$$RTK" go -C apps/api vet ./...; \
	cd apps/api && "$$RTK" "$(STATICCHECK)" ./...

ai-lint-frontend: ## AI: executa o lint do workspace web via RTK
	@command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	"$$RTK" npm --workspace @cumuru/web run lint

ai-build-frontend: ## AI: executa o build do workspace web via RTK
	@command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	"$$RTK" npm --workspace @cumuru/web run build

ai-check: ## AI: executa apenas o bundle rápido test-unit via RTK
	@command -v "$$RTK" >/dev/null 2>&1 || { echo "RTK command not found: $$RTK" >&2; exit 2; }; \
	"$$RTK" "$(MAKE)" --no-print-directory test-unit

images: ## Constrói as imagens OCI com metadata reprodutível via Docker
	@"$(WITH_BUILD_METADATA)" docker compose build api web

sbom: ## Gera SBOMs Go, npm e das imagens; requer Docker
	@mkdir -p artifacts/sbom
	cd apps/api && "$(CYCLONEDX_GOMOD)" mod \
		-json -output "$(CURDIR)/artifacts/sbom/api.cdx.json"
	npm run sbom:web
	@$(MAKE) image-sbom

image-sbom: ## Gera SBOM das imagens já construídas via scanner fixado
	@"$(WITH_BUILD_METADATA)" "$(IMAGE_ARTIFACTS)" sbom \
		"$(TRIVY_IMAGE)" "$(CUMURU_API_IMAGE)" "$(CUMURU_WEB_IMAGE)"

scanner-images: ## Materializa e confere scanners fixados por digest
	@"$(MATERIALIZE_PINNED_IMAGE)" "$(TRIVY_IMAGE)" "$(GITLEAKS_IMAGE)"

scan: scanner-images ## Executa govulncheck, npm audit, Trivy fs e Gitleaks
	cd apps/api && "$(GOVULNCHECK)" ./...
	npm audit --audit-level=high
	docker run --rm \
		--volume "$(CURDIR):/workspace:ro" \
		--workdir /workspace \
		"$(TRIVY_IMAGE)" fs \
		--exit-code 1 --severity HIGH,CRITICAL --no-progress .
	docker run --rm \
		--volume "$(CURDIR):/workspace:ro" \
		--workdir /workspace \
		"$(GITLEAKS_IMAGE)" detect \
		--source . --no-git --redact --exit-code 1

image-scan: ## Escaneia imagens já construídas sem montar o Docker socket
	@"$(WITH_BUILD_METADATA)" "$(IMAGE_ARTIFACTS)" scan \
		"$(TRIVY_IMAGE)" "$(CUMURU_API_IMAGE)" "$(CUMURU_WEB_IMAGE)"

compose-config: ## Valida o Compose base e o overlay local com metadata reprodutível
	@"$(WITH_BUILD_METADATA)" docker compose config --quiet
	@"$(WITH_BUILD_METADATA)" $(LOCAL_COMPOSE) config --quiet
	@"$(WITH_BUILD_METADATA)" $(LOCAL_COMPOSE) \
		-f deploy/compose.local-test.yaml config --quiet
	@LOCAL_E2E_PORT=4174 "$(WITH_BUILD_METADATA)" $(LOCAL_COMPOSE) \
		-f deploy/compose.phase4-full-stack.yaml \
		-f deploy/compose.local-e2e.yaml config --quiet

up: ## Sobe a demo local, aplica fixtures idempotentes e espera a publicação
	@"$(WITH_BUILD_METADATA)" $(LOCAL_COMPOSE) up --build --detach --wait

down: ## Para a stack Compose sem remover volumes
	@"$(WITH_BUILD_METADATA)" $(LOCAL_COMPOSE) down --remove-orphans

dev: ## Alias de up para a stack Compose comprovada; não é hot reload
	@$(MAKE) --no-print-directory up

docker-dev: ## Sobe a stack Docker com hot reload em 127.0.0.1:5173; projeto cumuru-dev
	@"$(WITH_BUILD_METADATA)" $(DEV_COMPOSE) up --build --detach --wait \
		--wait-timeout 300
	@echo "hot reload em http://127.0.0.1:5173"

docker-dev-down: ## Para a stack de hot reload; preserva volumes e a stack estática
	@"$(WITH_BUILD_METADATA)" $(DEV_COMPOSE) down --remove-orphans

docker-dev-status: ## Mostra o status da stack de hot reload
	@"$(WITH_BUILD_METADATA)" $(DEV_COMPOSE) ps --all

docker-dev-logs: ## Acompanha os logs da stack de hot reload; use DOCKER_DEV_SERVICES="api web"
	@set -eu; \
	set --; \
	for service in $(DOCKER_DEV_SERVICES); do \
		case "$$service" in \
			postgres|migrate|local-demo|api|worker|web) ;; \
			*) echo "invalid DOCKER_DEV_SERVICES entry: $$service" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$service"; \
	done; \
	"$(WITH_BUILD_METADATA)" $(DEV_COMPOSE) logs --follow --tail 50 "$$@"

dev-web: ## Inicia o Vite; pressupõe API local disponível para o login
	npm --workspace @cumuru/web run dev

docker-up: ## Alias não destrutivo de up
	@$(MAKE) --no-print-directory up

docker-down: ## Alias não destrutivo de down; preserva volumes
	@$(MAKE) --no-print-directory down

docker-status: ## Mostra status dos DOCKER_SERVICES validados ou de toda a stack
	@set -eu; \
	set --; \
	for service in $$DOCKER_SERVICES; do \
		case "$$service" in \
			postgres|migrate|local-demo|api|worker|web) ;; \
			*) echo "invalid DOCKER_SERVICES entry: $$service" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$service"; \
	done; \
	"$(WITH_BUILD_METADATA)" $(LOCAL_COMPOSE) ps --all "$$@"

docker-logs: ## Segue logs com DOCKER_LOG_TAIL e DOCKER_SERVICES validados
	@set -eu; \
	case "$$DOCKER_LOG_TAIL" in \
		''|*[!0-9]*) echo "DOCKER_LOG_TAIL must be an integer from 1 to 10000" >&2; exit 2 ;; \
	esac; \
	test "$$DOCKER_LOG_TAIL" -ge 1 && test "$$DOCKER_LOG_TAIL" -le 10000 || \
		{ echo "DOCKER_LOG_TAIL must be an integer from 1 to 10000" >&2; exit 2; }; \
	set --; \
	for service in $$DOCKER_SERVICES; do \
		case "$$service" in \
			postgres|migrate|local-demo|api|worker|web) ;; \
			*) echo "invalid DOCKER_SERVICES entry: $$service" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$service"; \
	done; \
	"$(WITH_BUILD_METADATA)" $(LOCAL_COMPOSE) logs --tail "$$DOCKER_LOG_TAIL" --follow "$$@"

docker-restart: ## Reinicia containers existentes dos DOCKER_SERVICES validados
	@set -eu; \
	set --; \
	for service in $$DOCKER_SERVICES; do \
		case "$$service" in \
			postgres|migrate|local-demo|api|worker|web) ;; \
			*) echo "invalid DOCKER_SERVICES entry: $$service" >&2; exit 2 ;; \
		esac; \
		set -- "$$@" "$$service"; \
	done; \
	"$(WITH_BUILD_METADATA)" $(LOCAL_COMPOSE) restart "$$@"

migrate-up: ## Aplica migrations na stack Compose local
	@"$(WITH_BUILD_METADATA)" docker compose run --rm migrate

migrate-down-local: ## Reverte uma migration local descartável; exige confirmação
	@test "$(ALLOW_DESTRUCTIVE_MIGRATION_DOWN)" = "yes" || \
		(echo "set ALLOW_DESTRUCTIVE_MIGRATION_DOWN=yes for a disposable local database" >&2; exit 2)
	"$(WITH_BUILD_METADATA)" docker compose run --rm migrate \
		-path=/migrations \
		-database=postgres://cumuru_migration:cumuru-local-migration-only@postgres:5432/cumuru?sslmode=disable \
		down 1

smoke: ## Executa o smoke da stack Compose local
	@LOCAL_FAKE_OIDC_TOKEN=cumuru-local-platform-read \
		SMOKE_PROFILE=local-demo bash deploy/scripts/smoke.sh

ci: ## Executa o gate completo sequencial; pesado, usa Docker e rede
	@$(MAKE) --no-print-directory openapi-lint
	@$(MAKE) --no-print-directory generated-check
	@$(MAKE) --no-print-directory migration-test
	@$(MAKE) --no-print-directory local-restore-drill
	@$(MAKE) --no-print-directory local-demo-test
	@$(MAKE) --no-print-directory phase2-integration
	@$(MAKE) --no-print-directory phase2-proxy-test
	@$(MAKE) --no-print-directory phase3-integration
	@$(MAKE) --no-print-directory phase3-proxy-test
	@$(MAKE) --no-print-directory phase4-integration
	@$(MAKE) --no-print-directory phase4-proxy-test
	@$(MAKE) --no-print-directory test
	@$(MAKE) --no-print-directory test-backend-race
	@$(MAKE) --no-print-directory typecheck
	@$(MAKE) --no-print-directory post-task-quality
	@$(MAKE) --no-print-directory infra-validation
	@$(MAKE) --no-print-directory build
	@$(MAKE) --no-print-directory images
	@$(MAKE) --no-print-directory phase2-full-stack
	@$(MAKE) --no-print-directory phase4-benchmark
	@$(MAKE) --no-print-directory local-demo-e2e
	@$(MAKE) --no-print-directory sbom
	@$(MAKE) --no-print-directory scan
	@$(MAKE) --no-print-directory image-scan
