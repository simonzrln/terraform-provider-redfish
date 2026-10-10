PKG_NAME=redfish
VERSION?=2.0.1
TEST?=$$(go list ./... | grep -v 'vendor')
INSTALL_ROOT?=~/.terraform.d/plugins
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)
HOSTNAME=registry.terraform.io
NAMESPACE=simonzrln
BINARY=terraform-provider-${PKG_NAME}
OS_ARCH=linux_amd64

default: build

build:
	go install
	go build -o $(CURDIR)/bin/${OS_ARCH}/${BINARY}_v$(VERSION)

# formats all .go files
fmt:
	@echo "==> Fixing source code with gofmt..."
	gofmt -s -w $(GOFMT_FILES)

# runs a Go format check
fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

errcheck:
	@sh -c "'$(CURDIR)/scripts/errcheck.sh'"

check:
	golangci-lint run --fix

lint:
	@echo "==> Checking source code against linters..."
	golangci-lint run --fix

# vets all .go files
vet:
	@echo "go vet ."
	@go vet $$(go list ./... ) ; if [ $$? -ne 0 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

gosec:
	gosec -exclude-generated ./...

# unsigned local build of all release archives into dist/; keep the image tag in sync with .github/workflows/release.yml
snapshot:
	docker run --rm -u "$$(id -u):$$(id -g)" -e HOME=/tmp -v "$(CURDIR):/src" -w /src \
		goreleaser/goreleaser:v2.18.2 release --snapshot --clean --skip=sign

install: build
	mkdir -p $(INSTALL_ROOT)/${HOSTNAME}/${NAMESPACE}/${PKG_NAME}/${VERSION}/${OS_ARCH}
	mv $(CURDIR)/bin/${OS_ARCH}/${BINARY}_v$(VERSION) $(INSTALL_ROOT)/${HOSTNAME}/${NAMESPACE}/${PKG_NAME}/${VERSION}/${OS_ARCH}

test:
	go test ./redfish/... $(TESTARGS)

# regenerates docs/ (tfplugindocs via the go:generate line in main.go)
docs generate:
	GOFLAGS="-mod=readonly" go generate ./...

testacc:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

clean:
	go clean --cache
	rm -rf vendor bin

.PHONY: default build install snapshot test testacc vet fmt fmtcheck errcheck check lint gosec docs generate clean
