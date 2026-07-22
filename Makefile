TEMPLATE_VERSION ?= 0.2.0
TEMPLATE_ARCHIVE ?= ../golang-clean-architecture/dist/golang-clean-architecture-template-$(TEMPLATE_VERSION).tar.gz
TEMPLATE_CHECKSUM ?= $(TEMPLATE_ARCHIVE).sha256

.PHONY: build test vet check update-template

build:
	go build ./cmd/go-craft

test:
	go test ./...

vet:
	go vet ./...

check: test vet

update-template:
	test -f "$(TEMPLATE_ARCHIVE)"
	test -f "$(TEMPLATE_CHECKSUM)"
	cd "$$(dirname "$(TEMPLATE_ARCHIVE)")" && sha256sum -c "$$(basename "$(TEMPLATE_CHECKSUM)")"
	rm -rf internal/scaffold/_assets/template
	mkdir -p internal/scaffold/_assets/template
	tar -xzf "$(TEMPLATE_ARCHIVE)" -C internal/scaffold/_assets/template
	mv internal/scaffold/_assets/template/go.mod internal/scaffold/_assets/template/go.mod.tmpl
	gofmt -w internal/scaffold/_assets/template
