.PHONY: build clean

BINARY_NAME := helloworld

build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o $(BINARY_NAME) main.go

clean:
	rm -f $(BINARY_NAME)

