APP := $(notdir $(shell go list -m))
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)

.PHONY: build check clean format install-hooks pre-commit run test tidy vet

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) .

check: format tidy vet test

clean:
	rm -rf $(BIN_DIR)

format:
	go fmt ./...

install-hooks:
	pre-commit install

pre-commit:
	pre-commit run --all-files

run:
	go run .

test:
	go test ./...

tidy:
	go mod tidy

vet:
	go vet ./...
