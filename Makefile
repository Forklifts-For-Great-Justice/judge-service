.PHONY: all build test test-verbose run schema clean help

# Variables
BINARY_NAME=judge
MAIN_PATH=./cmd/judge
GENSCHEMA_PATH=./cmd/genschema

all: build

## 🔨 build: Build the judge-service binary
build:
	@echo "🛠️  Building $(BINARY_NAME)..."
	@go build -o bin/$(BINARY_NAME) $(MAIN_PATH)
	@echo "✅ Build complete! Binary generated at bin/$(BINARY_NAME) 🚀"

## 🧪 test: Run all tests
test:
	@echo "🧪 Running unit tests..."
	@go test ./...
	@echo "🎉 All tests passed successfully! 🏆"

## 🔬 test-verbose: Run all tests with verbose output
test-verbose:
	@echo "🔬 Running verbose unit tests..."
	@go test -v ./...
	@echo "🎉 All tests passed successfully! 🏆"

## 🚀 run: Run the service locally
run:
	@echo "⚡ Starting judge-service locally..."
	@go run $(MAIN_PATH)

## 📋 schema: Regenerate OpenAPI specification
schema:
	@echo "📝 Generating OpenAPI schema..."
	@go run $(GENSCHEMA_PATH)
	@echo "✨ OpenAPI schema generation complete! 📄"

## 🧹 clean: Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf bin/ $(BINARY_NAME)
	@echo "✨ Cleanup complete! 🗑️"

## 👥 set-teams: Set teams 'foo' and 'bar' for the current round
set-teams:
	@echo "🎮 Setting teams 'foo' and 'bar' in current round..."
	@python3 ./scripts/set_current_round_teams.py


## ❓ help: Display available targets
help:
	@echo "⚖️  Judge Service Makefile"
	@echo "-------------------------"
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
