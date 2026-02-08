.PHONY: build install clean test run demo help

# Default target
all: build

# Build the application
build:
	@echo "Building wydo..."
	@go build -o wydo
	@echo "Done: Build complete: ./wydo"

# Install to system PATH
install:
	@echo "Installing wydo..."
	@go install
	@echo "Done: Installed to ~/go/bin/wydo"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f wydo
	@go clean
	@echo "Done: Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@go test ./...
	@echo "Done: Tests complete"

# Run the application
run: build
	@./wydo

# Run with test data
demo: build
	@./wydo -d testdata -r testdata

# Show help
help:
	@echo "Wydo - Unified Agenda App"
	@echo ""
	@echo "Available targets:"
	@echo "  make build    - Build the application (default)"
	@echo "  make install  - Install to ~/go/bin"
	@echo "  make clean    - Remove build artifacts"
	@echo "  make test     - Run tests"
	@echo "  make run      - Build and run the application"
	@echo "  make demo     - Build and run with test data"
	@echo "  make help     - Show this help message"
