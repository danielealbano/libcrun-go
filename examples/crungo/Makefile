.PHONY: build test test-unit test-integration clean

BINARY_NAME := crungo

build:
	go build -o $(BINARY_NAME) .

test-unit:
	go test -v ./...

test-integration:
	@echo "Running integration tests (requires root and network access)..."
	sudo go test -tags=integration -v ./...

test: test-unit

clean:
	rm -f $(BINARY_NAME)
	go clean ./...

