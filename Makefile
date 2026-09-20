# HiveStack Makefile
# KVM Virtualization Management Platform

.PHONY: all build test lint sbom vuln clean

# Default target
all: build test

# Build all binaries
build:
	go build -o bin/hive ./cmd/hive
	go build -o bin/hive-manager ./cmd/hive-manager
	go build -o bin/hive-node ./node/

# Run tests with race detection and coverage
test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | grep total

# Run linter
lint:
	golangci-lint run ./...

# Generate SBOM with syft
sbom:
	syft dir:. -o spdx-json > sbom.json
	syft dir:. -o cyclonedx-json > sbom-cyclonedx.json

# Run vulnerability scan
vuln:
	govulncheck ./...
	trivy fs --scanners vuln,secret,config .

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out sbom.json sbom-cyclonedx.json

# Run all CI checks locally
ci: lint vuln test sbom
	@echo "All CI checks passed!"

# Install development tools
install-tools:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/anchore/syft/cmd/syft@latest
	go install github.com/aquasecurity/trivy@latest
	go install github.com/oasdiff/oasdiff@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install sigs.k8s.io/cosign@latest
	@echo "All development tools installed"

# Run integration tests
test-integration:
	go test ./tests/... -v -tags=integration -timeout=15m

# Run unit tests only
test-unit:
	go test ./internal/... ./pkg/... -v -race -coverprofile=coverage.out -covermode=atomic

# Check coverage threshold (70%)
coverage-check:
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $${COVERAGE}%"; \
	if (( $$(echo "$${COVERAGE} < 70" | bc -l) )); then \
		echo "ERROR: Coverage $${COVERAGE}% is below 70% threshold"; \
		exit 1; \
	fi

# Validate OpenAPI spec
validate-api:
	spectral lint api/openapi.yaml

# Check for OpenAPI breaking changes
check-api-breaking:
	oasdiff breaking https://raw.githubusercontent.com/maddydevel/HiveStack/main/api/openapi.yaml api/openapi.yaml

# Build container image
build-image:
	docker build -t hivestack:dev -f appliance/Dockerfile .

# Scan container image
scan-image: build-image
	trivy image --scanners vuln hivestack:dev

# Sign container image with cosign
sign-image: build-image
	cosign sign --yes hivestack:dev

# Generate and sign SBOM for container image
sbom-image: build-image
	syft hivestack:dev -o spdx-json > sbom-image.json
	cosign attach sbom --sbom sbom-image.json hivestack:dev
