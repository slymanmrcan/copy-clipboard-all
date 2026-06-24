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

# Pre-built binary indirmeyi dene (Go gerektirmez)
ASSET_NAME="cb-${OS}-${ARCH}"
BINARY_URL="https://github.com/$REPO/releases/latest/download/$ASSET_NAME"
CHECKSUM_URL="https://github.com/$REPO/releases/latest/download/checksums.txt"
info "Pre-built binary indiriliyor: $BINARY_URL"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

DOWNLOAD_SUCCESS=false
if command -v curl &>/dev/null; then
  if curl -fsSL "$BINARY_URL" -o "$TMP/$BINARY" &&
     curl -fsSL "$CHECKSUM_URL" -o "$TMP/checksums.txt"; then
    DOWNLOAD_SUCCESS=true
  fi
elif command -v wget &>/dev/null; then
  if wget -qO "$TMP/$BINARY" "$BINARY_URL" &&
     wget -qO "$TMP/checksums.txt" "$CHECKSUM_URL"; then
    DOWNLOAD_SUCCESS=true
  fi
fi

if [ "$DOWNLOAD_SUCCESS" = "true" ]; then
  EXPECTED_CHECKSUM=$(awk -v asset="$ASSET_NAME" '$2 == asset || $2 == "*" asset {print $1}' "$TMP/checksums.txt")
  if ! [[ "$EXPECTED_CHECKSUM" =~ ^[a-fA-F0-9]{64}$ ]]; then
    warn "Release checksum kaydı bulunamadı veya geçersiz."
    DOWNLOAD_SUCCESS=false
  elif command -v sha256sum &>/dev/null; then
    ACTUAL_CHECKSUM=$(sha256sum "$TMP/$BINARY" | awk '{print $1}')
  elif command -v shasum &>/dev/null; then
    ACTUAL_CHECKSUM=$(shasum -a 256 "$TMP/$BINARY" | awk '{print $1}')
  else
    warn "SHA-256 doğrulaması için sha256sum veya shasum bulunamadı."
    DOWNLOAD_SUCCESS=false
  fi

  if [ "$DOWNLOAD_SUCCESS" = "true" ] && [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
    warn "İndirilen binary checksum doğrulamasından geçemedi."
    DOWNLOAD_SUCCESS=false
  fi
fi

if [ "$DOWNLOAD_SUCCESS" = "true" ]; then
  info "SHA-256 checksum doğrulandı."
  if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
  else
    sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
  fi
  chmod +x "$INSTALL_DIR/$BINARY"
  info "✓ $BINARY $INSTALL_DIR dizinine kuruldu (pre-built)!"
else
  warn "Doğrulanmış pre-built binary kullanılamadı; kaynak koddan derleme denenecek."
  
  if command -v go &>/dev/null; then
    info "Go bulundu, kaynak koddan derleniyor..."
    
    # GitHub'dan kaynak indir
    if command -v curl &>/dev/null; then
      curl -fsSL "https://github.com/$REPO/archive/refs/heads/main.tar.gz" -o "$TMP/cb.tar.gz"
    elif command -v wget &>/dev/null; then
      wget -qO "$TMP/cb.tar.gz" "https://github.com/$REPO/archive/refs/heads/main.tar.gz"
    else
      errxit "curl veya wget bulunamadı"
    fi

    tar -xzf "$TMP/cb.tar.gz" -C "$TMP"
    cd "$TMP"/*/ # dinamik olarak çıkarılan klasöre gir
    
    go build -ldflags="-s -w" -o "$TMP/$BINARY" .

    if [ -w "$INSTALL_DIR" ]; then
      mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
    else
      sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
    fi

    chmod +x "$INSTALL_DIR/$BINARY"
    info "✓ $BINARY $INSTALL_DIR dizinine kuruldu (derlendi)!"
  else
    warn "Go bulunamadı. Lütfen Go'yu kurun: https://go.dev/dl/"
    warn "veya manuel olarak bir binary dosyası yükleyin."
    exit 1
  fi
fi

echo ""
info "Kurulum tamamlandı! Kullanım:"
echo "  cb \"metin\"          # metni kopyala"
echo "  cb dosya.txt        # dosya kopyala"
echo "  ls | cb             # komut çıktısını kopyala"
echo "  cb -p               # clipboard'ı ekrana yaz"
echo ""

# Linux kullanıcısına xclip hatırlat (SSH/Headless dışında)
if [ "$OS" = "linux" ]; then
  if [ -n "$SSH_CLIENT" ] || [ -n "$SSH_TTY" ] || [ -z "$DISPLAY" ]; then
    info "SSH/Headless ortam algılandı: OSC 52 kullanılacaktır, xclip/xsel kurmanıza gerek yoktur."
  else
    if ! command -v xclip &>/dev/null && ! command -v xsel &>/dev/null && ! command -v wl-copy &>/dev/null; then
      warn "Yerel masaüstü (GUI) ortamında kopyalama için xclip gereklidir:"
      warn "  sudo apt install xclip   (Debian/Ubuntu)"
      warn "  sudo pacman -S xclip     (Arch)"
    fi
  fi
fi
