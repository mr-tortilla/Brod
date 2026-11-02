# Версия берётся из git-тега: v0.1.0 → 0.1.0, между релизами — 0.1.0-3-gabc1234, с правками — ...-dirty.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
LDFLAGS := -X main.version=$(VERSION)

.PHONY: dev build build-windows installer-windows packages test icons licenses version

# версия не подставляется: у dev-сборки свой ключ единственного экземпляра (см. lockID в main.go),
# поэтому её можно запускать рядом с установленным Brod
dev:
	WAILS_DEV_ALLOWED_HOSTS=wails.localhost wails dev

build:
	wails build -ldflags "$(LDFLAGS)"

build-windows:
	wails build -platform windows/amd64 -ldflags "$(LDFLAGS)"

# Установщик для Windows (NSIS). Нужен makensis: sudo apt install nsis
installer-windows:
	wails build -platform windows/amd64 -nsis -ldflags "$(LDFLAGS)"

# Linux-пакеты: .deb (Ubuntu/Debian/Mint), .rpm (Fedora/openSUSE), .pkg.tar.zst (Arch).
# Нужен nfpm: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
packages: build
	mkdir -p build/dist
	for p in deb rpm archlinux; do VERSION=$(VERSION) nfpm package -f nfpm.yaml -p $$p -t build/dist/ || exit 1; done

test:
	go test -tags webkit2_41,with_utls,with_gvisor,badlinkname ./...

# после изменения зависимостей: пересобрать THIRD_PARTY_LICENSES.txt
licenses:
	go run tools/genlicenses/main.go

icons:
	go run -mod=mod tools/genicons/main.go

version:
	@echo $(VERSION)
