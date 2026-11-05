PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
DESTDIR ?=
SUDO ?=

.PHONY: build install test test-integration

build:
	mkdir -p bin
	go build -ldflags="-s -w" -o bin/capg .

test:
	go test ./...

test-integration:
	go test -tags=integration -run TestPostgres18 -v ./...

install: build
	@printf 'Installing to %s. System directories may require sudo; use make install SUDO=sudo if needed.\n' '$(DESTDIR)$(BINDIR)'
	$(SUDO) install -d '$(DESTDIR)$(BINDIR)'
	$(SUDO) install -m 0755 bin/capg '$(DESTDIR)$(BINDIR)/capg'
