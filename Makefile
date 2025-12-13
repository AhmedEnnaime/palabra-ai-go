.PHONY: help setup run test test-langs build clean

help:
	@echo "Palabra.ai Translation Service - Available Commands:"
	@echo ""
	@echo "  make setup       - Install dependencies and create .env file"
	@echo "  make run         - Run the application"
	@echo "  make test        - Run integration tests"
	@echo "  make test-langs  - Test multiple language pairs"
	@echo "  make build       - Build the binary"
	@echo "  make clean       - Clean build artifacts"
	@echo ""

setup:
	@echo "Setting up project..."
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "✓ Created .env file from .env.example"; \
		echo "⚠️  Please edit .env and add your credentials"; \
	else \
		echo "✓ .env file already exists"; \
	fi
	@echo "Installing dependencies..."
	go mod download
	@echo "✓ Setup complete!"

run:
	@echo "Starting Palabra Translation Service..."
	go run cmd/main.go

run-custom:
	@echo "Starting with custom languages..."
	go run cmd/main.go -source=$(SOURCE) -target=$(TARGET)

test:
	@echo "Running integration tests..."
	go run cmd/main.go --test

test-langs:
	@echo "Testing multiple language pairs..."
	go run cmd/main.go --test-langs

build:
	@echo "Building binary..."
	@mkdir -p bin
	go build -o bin/palabra-translation cmd/main.go
	@echo "✓ Binary created at bin/palabra-translation"

build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/palabra-translation-linux-amd64 cmd/main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/palabra-translation-darwin-amd64 cmd/main.go
	GOOS=windows GOARCH=amd64 go build -o bin/palabra-translation-windows-amd64.exe cmd/main.go
	@echo "✓ Built for Linux, macOS, and Windows"

clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f *.log
	@echo "✓ Clean complete"

install: build
	@echo "Installing to /usr/local/bin..."
	sudo cp bin/palabra-translation /usr/local/bin/
	@echo "✓ Installed successfully"

run-es:
	go run cmd/main.go -source=en -target=es

run-fr:
	go run cmd/main.go -source=en -target=fr

run-de:
	go run cmd/main.go -source=en -target=de