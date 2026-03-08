PACK            := improvmx
PROJECT         := github.com/lokkju/pulumi-improvmx
PROVIDER        := pulumi-resource-${PACK}
VERSION_PATH    := ${PROJECT}/provider.Version
PROVIDER_VERSION ?= 0.1.0-alpha.0+dev
VERSION_GENERIC  = $(shell pulumictl convert-version --language generic --version "$(PROVIDER_VERSION)" 2>/dev/null || echo "$(PROVIDER_VERSION)")
SCHEMA_FILE     := schema.json
WORKING_DIR     := $(shell pwd)

.PHONY: provider
provider:
	go build -o $(WORKING_DIR)/bin/${PROVIDER} \
		-ldflags "-X ${VERSION_PATH}=${VERSION_GENERIC}" \
		$(PROJECT)/provider/cmd/$(PROVIDER)

$(SCHEMA_FILE): provider
	pulumi package get-schema $(WORKING_DIR)/bin/${PROVIDER} | jq -f $(WORKING_DIR)/scripts/patch-schema.jq > $(SCHEMA_FILE)

.PHONY: codegen
codegen: $(SCHEMA_FILE) sdk/python sdk/nodejs sdk/go sdk/dotnet

sdk/%: $(SCHEMA_FILE)
	rm -rf $@
	pulumi package gen-sdk --language $* $(SCHEMA_FILE) --version "${VERSION_GENERIC}"

sdk/python: $(SCHEMA_FILE)
	rm -rf $@
	pulumi package gen-sdk --language python $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	cp README.md sdk/python/ 2>/dev/null || true

sdk/go: $(SCHEMA_FILE)
	rm -rf $@
	pulumi package gen-sdk --language go $(SCHEMA_FILE) --version "${VERSION_GENERIC}"

.PHONY: test
test:
	go test -v -count=1 -cover -timeout 2h ./provider/...

.PHONY: test_integration
test_integration:
	IMPROVMX_LIVE_TEST=1 go test -v -count=1 -run TestLive -timeout 5m ./provider/...

.PHONY: lint
lint:
	golangci-lint run ./provider/...

.PHONY: install
install: provider
	cp $(WORKING_DIR)/bin/${PROVIDER} $(GOPATH)/bin/

.PHONY: clean
clean:
	rm -rf bin/ $(SCHEMA_FILE) sdk/python sdk/nodejs sdk/dotnet sdk/java
