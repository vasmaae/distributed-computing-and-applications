GOLANGCI_LINT_VERSION := v2.14.0
SWAG_VERSION := v1.16.6

BIN_DIR := $(CURDIR)/bin

GOLANGCI_LINT := $(BIN_DIR)/golangci-lint
SWAG := $(BIN_DIR)/swag

SWAG_MAIN := ./cmd/api/main.go
SWAG_DOCS_DIR := ./docs
MIGRATIONS_DIR := internal/migrations/sql
GORM_MODELS_PATH := ./internal/repository/entity

.PHONY: \
	install-golangci-lint \
	lint \
	install-swag \
	swagger \
	tests \
	e2e-tests \
	migration


install-golangci-lint:
	@mkdir -p $(BIN_DIR)
	@if [ ! -f "$(GOLANGCI_LINT)" ]; then \
		echo "📦 Устанавливаем golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		GOBIN=$(BIN_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi


lint: install-golangci-lint install-swag
	@$(SWAG) fmt
	@$(GOLANGCI_LINT) run ./... --config=.golangci.yml --fix


install-swag:
	@mkdir -p $(BIN_DIR)
	@if [ ! -f "$(SWAG)" ]; then \
		echo "📦 Устанавливаем swag $(SWAG_VERSION)..."; \
		GOBIN=$(BIN_DIR) go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION); \
	fi


swagger: install-swag
	@echo "📚 Генерируем Swagger документацию..."
	@$(SWAG) init \
		-g $(SWAG_MAIN) \
		-o $(SWAG_DOCS_DIR) \
		--parseInternal \
		--parseDependencyLevel 1

tests:
	@go test ./internal/... -v


e2e-tests:
	@go test ./internal/tests/... -v -run TestAPI


migration:
	@test -n "$(name)" || (echo "usage: make migration name=create_employees" && exit 1)
	@tmp=$$(mktemp /tmp/gorm-schema.XXXXXX.sql); \
	trap 'rm -f "$$tmp"' EXIT; \
	echo "📦 Loading GORM schema..."; \
	go run -mod=mod ariga.io/atlas-provider-gorm load \
		--path "$(GORM_MODELS_PATH)" \
		--dialect postgres \
		> "$$tmp"; \
	echo "🗃 Generating migration $(name)..."; \
	atlas migrate diff "$(name)" \
		--dir "file://$(MIGRATIONS_DIR)" \
		--to "file://$$tmp" \
		--dev-url "docker://postgres/16/dev?search_path=public"