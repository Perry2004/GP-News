APP := $(notdir $(shell go list -m))
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)

.PHONY: build check clean format install-hooks pre-commit run test cover tidy vet

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
	go test -v ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

tidy:
	go mod tidy

vet:
	go vet ./...
