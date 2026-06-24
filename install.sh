#!/bin/bash
# cb kurulum scripti
# Kullanım: curl -fsSL https://raw.githubusercontent.com/slymanmrcan/copy-clipboard-all/main/install.sh | bash

set -e

REPO="slymanmrcan/copy-clipboard-all"
BINARY="cb"
INSTALL_DIR="/usr/local/bin"

# Renk kodları
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()    { echo -e "${GREEN}[cb]${NC} $1"; }
warn()    { echo -e "${YELLOW}[cb]${NC} $1"; }
errxit()  { echo -e "${RED}[hata]${NC} $1"; exit 1; }

# OS ve arch tespit
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) errxit "Desteklenmeyen mimari: $ARCH" ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) errxit "Desteklenmeyen OS: $OS (Windows için README'ye bak)" ;;
esac

info "OS: $OS / Arch: $ARCH tespit edildi"

# Kaynak koddan derle (go yüklüyse)
if command -v go &>/dev/null; then
  info "Go bulundu, kaynak koddan derleniyor..."

  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT

  # GitHub'dan kaynak indir
  if command -v curl &>/dev/null; then
    curl -fsSL "https://github.com/$REPO/archive/refs/heads/main.tar.gz" -o "$TMP/cb.tar.gz"
  elif command -v wget &>/dev/null; then
    wget -qO "$TMP/cb.tar.gz" "https://github.com/$REPO/archive/refs/heads/main.tar.gz"
  else
    errxit "curl veya wget bulunamadı"
  fi

  tar -xzf "$TMP/cb.tar.gz" -C "$TMP"
  cd "$TMP"/cb-main

  go build -ldflags="-s -w" -o "$TMP/$BINARY" .

  if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
  else
    sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
  fi

  chmod +x "$INSTALL_DIR/$BINARY"
  info "✓ $BINARY $INSTALL_DIR dizinine kuruldu!"

else
  warn "Go bulunamadı. Lütfen Go'yu kurun: https://go.dev/dl/"
  warn "veya 'go build -o cb .' komutuyla manuel derleyin."
  exit 1
fi

echo ""
info "Kurulum tamamlandı! Kullanım:"
echo "  cb \"metin\"          # metni kopyala"
echo "  cb dosya.txt        # dosya kopyala"
echo "  ls | cb             # komut çıktısını kopyala"
echo "  cb -p               # clipboard'ı ekrana yaz"
echo ""

# Linux kullanıcısına xclip hatırlat
if [ "$OS" = "linux" ]; then
  if ! command -v xclip &>/dev/null && ! command -v xsel &>/dev/null && ! command -v wl-copy &>/dev/null; then
    warn "Linux'ta clipboard için xclip gerekli:"
    warn "  sudo apt install xclip   (Debian/Ubuntu)"
    warn "  sudo pacman -S xclip     (Arch)"
  fi
fi