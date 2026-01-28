default: install

# Run acceptance tests
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

# Build the provider
.PHONY: build
build:
	go build -o terraform-provider-netbox

# Install the provider locally for testing
.PHONY: install
install: build
	mkdir -p ~/.terraform.d/plugins/local/justinr/netbox/0.0.1/linux_amd64
	mv terraform-provider-netbox ~/.terraform.d/plugins/local/justinr/netbox/0.0.1/linux_amd64/

# Run unit tests
.PHONY: test
test:
	go test -v -cover -timeout=120s -parallel=4 ./...

# Run linter
.PHONY: lint
lint:
	golangci-lint run

# Format code
.PHONY: fmt
fmt:
	gofmt -s -w -e .

# Generate documentation
.PHONY: generate
generate:
	go generate ./...

# Clean build artifacts
.PHONY: clean
clean:
	rm -f terraform-provider-netbox
	rm -rf dist/

# Download dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy
