APP := $(notdir $(shell go list -m))
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)
EMAIL_TEMPLATE_DIR := email/template
PNPM := pnpm

.PHONY: build check clean cover email-template-check email-template-export email-template-format email-template-install email-template-typecheck format go-check go-format install-hooks pre-commit run test tidy vet

build: email-template-export
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) .

check: go-check email-template-check

go-check: go-format tidy vet test

clean:
	rm -rf $(BIN_DIR) $(EMAIL_TEMPLATE_DIR)/out

format: go-format email-template-format

go-format:
	go fmt ./...

email-template-install:
	cd $(EMAIL_TEMPLATE_DIR) && $(PNPM) install --frozen-lockfile

email-template-format:
	cd $(EMAIL_TEMPLATE_DIR) && $(PNPM) check

email-template-typecheck:
	cd $(EMAIL_TEMPLATE_DIR) && $(PNPM) typecheck

email-template-check: email-template-format email-template-typecheck

email-template-export: email-template-install email-template-typecheck
	cd $(EMAIL_TEMPLATE_DIR) && $(PNPM) export

install-hooks:
	pre-commit install

pre-commit:
	pre-commit run --all-files

run: email-template-export
	go run .

test:
	go test -v ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

tidy:
	go mod tidy

vet:
	go vet ./...
