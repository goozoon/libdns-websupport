# Simple task runner for Linux/macOS

BINARY=libdns-websupport

.PHONY: build run test cert dns-test acme-test clean

build:
	go build .

run:
	go run .

test:
	go test ./...

cert: build
	./$(BINARY) create-cert

# Requires WEBSUPPORT_API_KEY and WEBSUPPORT_API_SECRET to be set
# Optional: WEBSUPPORT_TEST_ZONE (default: example.com)
# Optional: WEBSUPPORT_TEST_DOMAIN (default: libdns.example.com)
dns-test: build
	./$(BINARY) test

acme-test: build
	./$(BINARY) acme-test

clean:
	rm -f $(BINARY)
