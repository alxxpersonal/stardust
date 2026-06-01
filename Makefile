BINARY := stardust
BUILD_DIR := ./build
CMD_DIR := ./cmd/stardust

.PHONY: build run test lint fmt vet clean install changelog help

help: ## show help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## build the binary
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)

run: ## run from source
	go run $(CMD_DIR)

test: ## run tests with race detector
	go test -race -count=1 ./...

vet: ## run go vet
	go vet ./...

lint: vet ## run go vet + golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

fmt: ## format code
	gofmt -s -w $(shell find . -name '*.go' -not -path './build/*')

clean: ## remove build artifacts
	rm -rf $(BUILD_DIR)

install: build ## install the binary to GOBIN
	go install $(CMD_DIR)

changelog: ## regenerate CHANGELOG.md
	@command -v git-cliff >/dev/null 2>&1 && git-cliff -o CHANGELOG.md && echo "changelog updated" || echo "git-cliff not installed"
