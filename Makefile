# Makefile for cb (clipboard CLI tool)

BINARY_NAME=cb
INSTALL_DIR=/usr/local/bin

.PHONY: all build install uninstall clean test help

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
	rm -f $(BINARY_NAME)

test:
	@echo "Testler çalıştırılıyor / Running tests..."
	go test -v ./...

help:
	@echo "Kullanılabilir komutlar / Available commands:"
	@echo "  make          - Derler (build)"
	@echo "  make build    - Derler (build)"
	@echo "  make install  - Sistem seviyesinde kurar (/usr/local/bin)"
	@echo "  make uninstall- Sistem seviyesinden kaldırır"
	@echo "  make clean    - Derleme çıktılarını temizler"
	@echo "  make test     - Testleri çalıştırır"
