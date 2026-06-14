APP := $(notdir $(shell go list -m))
BIN_DIR := bin
CACHE_DIR ?= cache
BIN := $(BIN_DIR)/$(APP)
EMAIL_TEMPLATE_DIR := email/template
GOLANGCI_LINT ?= golangci-lint
PNPM := pnpm
DOCKER ?= docker
CONTAINER_IMAGE ?= gpnews:local
LAMBDA_IMAGE ?= gpnews-lambda:test
LAMBDA_PLATFORM ?= linux/amd64
LAMBDA_PORT ?= 9000
LAMBDA_TIMEOUT ?= 900
LAMBDA_EVENT ?= {}
ENV_FILE ?= .env
DOCKER_ENV_FILE := $(if $(wildcard $(ENV_FILE)),--env-file $(ENV_FILE),)

.PHONY: build build-container build-lambda check clean cover email-template-check email-template-export email-template-format email-template-install email-template-typecheck format go-check go-format install-hooks lint pre-commit run run-container run-lambda test tidy

build: email-template-export
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) ./cmd/local

build-container:
	$(DOCKER) buildx build --target local -t $(CONTAINER_IMAGE) .

build-lambda:
	$(DOCKER) buildx build --target lambda --platform $(LAMBDA_PLATFORM) --provenance=false -t $(LAMBDA_IMAGE) .

check: email-template-check go-check

go-check: go-format tidy lint test

clean:
	rm -rf $(BIN_DIR) $(EMAIL_TEMPLATE_DIR)/out
	rm -rf $(CACHE_DIR)

format: go-format email-template-format

go-format:
	goimports -local github.com/Perry2004/GP-News -w .

lint:
	$(GOLANGCI_LINT) run ./...

email-template-install:
	cd $(EMAIL_TEMPLATE_DIR) && $(PNPM) install --frozen-lockfile

email-template-format:
	cd $(EMAIL_TEMPLATE_DIR) && $(PNPM) check

email-template-typecheck:
	cd $(EMAIL_TEMPLATE_DIR) && $(PNPM) typecheck

email-template-check: email-template-format email-template-typecheck email-template-export

email-template-export: email-template-install email-template-typecheck
	cd $(EMAIL_TEMPLATE_DIR) && $(PNPM) export

install-hooks:
	pre-commit install

pre-commit:
	pre-commit run --all-files

run: email-template-export
	go run ./cmd/local

run-container: build-container
	$(DOCKER) run --rm $(DOCKER_ENV_FILE) $(CONTAINER_IMAGE)

run-lambda: build-lambda
	@container_id=$$($(DOCKER) run -d --platform $(LAMBDA_PLATFORM) -p $(LAMBDA_PORT):8080 -e AWS_LAMBDA_FUNCTION_TIMEOUT=$(LAMBDA_TIMEOUT) $(DOCKER_ENV_FILE) --entrypoint /usr/local/bin/aws-lambda-rie $(LAMBDA_IMAGE) ./gpnews-lambda); \
	trap '$(DOCKER) rm -f $$container_id >/dev/null' EXIT; \
	for _ in $$(seq 1 30); do \
		if curl -fsS "http://localhost:$(LAMBDA_PORT)/2015-03-31/functions/function/invocations" -d '$(LAMBDA_EVENT)'; then \
			echo; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "Lambda container did not respond on port $(LAMBDA_PORT)" >&2; \
	$(DOCKER) logs $$container_id >&2; \
	exit 1

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

tidy:
	go mod tidy
