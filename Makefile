# Makefile for cb (clipboard CLI tool)

BINARY_NAME=cb
INSTALL_DIR=/usr/local/bin
COVERAGE_FILE?=coverage.out
COVERAGE_THRESHOLD?=85.0

.PHONY: all build install uninstall clean test coverage lint help

all: build

build:
	@echo "Derleniyor / Building..."
	go build -ldflags="-s -w" -o $(BINARY_NAME) .

install: build
	@echo "Kuruluyor / Installing to $(INSTALL_DIR)..."
	@if [ -w $(INSTALL_DIR) ]; then \
		install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME); \
	else \
		echo "Yetki hatası. Lütfen 'sudo make install' kullanın veya $(INSTALL_DIR) için yazma yetkisini kontrol edin."; \
		sudo install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME); \
	fi
	@echo "✓ $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME) konumuna kuruldu!"

uninstall:
	@echo "Kaldırılıyor / Uninstalling from $(INSTALL_DIR)..."
	@if [ -w $(INSTALL_DIR) ]; then \
		rm -f $(INSTALL_DIR)/$(BINARY_NAME); \
	else \
		sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME); \
	fi
	@echo "✓ $(BINARY_NAME) kaldırıldı!"

clean:
	@echo "Temizleniyor / Cleaning..."
	rm -f $(BINARY_NAME) $(COVERAGE_FILE)

test:
	@echo "Testler çalıştırılıyor / Running tests..."
	go test -race -v ./...

coverage:
	@echo "Coverage testi çalıştırılıyor / Running coverage gate..."
	go test -race -covermode=atomic -coverprofile=$(COVERAGE_FILE) ./...
	@coverage=$$(go tool cover -func=$(COVERAGE_FILE) | awk '/^total:/ {gsub("%", "", $$3); print $$3}'); \
	echo "Coverage: $$coverage% (minimum: $(COVERAGE_THRESHOLD)%)"; \
	awk -v coverage="$$coverage" -v threshold="$(COVERAGE_THRESHOLD)" 'BEGIN { \
		if (coverage + 0 < threshold + 0) { \
			printf "Coverage threshold failed: %.1f%% < %.1f%%\n", coverage, threshold; \
			exit 1; \
		} \
	}'

lint:
	@echo "Lint kontrolü yapılıyor / Running linter..."
	golangci-lint run ./...

help:
	@echo "Kullanılabilir komutlar / Available commands:"
	@echo "  make          - Derler (build)"
	@echo "  make build    - Derler (build)"
	@echo "  make install  - Sistem seviyesinde kurar (/usr/local/bin)"
	@echo "  make uninstall- Sistem seviyesinden kaldırır"
	@echo "  make clean    - Derleme çıktılarını temizler"
	@echo "  make test     - Race detector ile testleri çalıştırır"
	@echo "  make coverage - Coverage üretir ve minimum $(COVERAGE_THRESHOLD)% eşiğini kontrol eder"
	@echo "  make lint     - Lint kontrolü yapar (golangci-lint)"
