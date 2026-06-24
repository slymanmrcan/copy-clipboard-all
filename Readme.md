# cb — clipboard CLI tool

Komut satırından clipboard'ı yönet. Linux, macOS ve Windows'ta çalışır.

## Kurulum

### Otomatik (Linux/macOS)
```bash
curl -fsSL https://raw.githubusercontent.com/slymanmrcan/copy-clipboard-all/main/install.sh | bash
```

### Manuel
```bash
git clone https://github.com/slymanmrcan/copy-clipboard-all
cd copy-clipboard-all
go build -o cb .
sudo mv cb /usr/local/bin/
```

### Windows
```powershell
git clone https://github.com/slymanmrcan/copy-clipboard-all
cd copy-clipboard-all
go build -o cb.exe .
# cb.exe'yi PATH'deki bir dizine taşı
```

## Kullanım

```bash
# Metin kopyala
cb "merhaba dünya"

# Birden fazla kelime (tırnak gerekmez)
cb merhaba dünya

# Dosya içeriğini kopyala
cb dosya.txt

# Pipe — komut çıktısını kopyala
ls -la | cb
git log --oneline | cb
cat /etc/hosts | cb

# Stdin (yönlendirme)
cb < dosya.txt

# Clipboard'ı ekrana yaz
cb -p

# Clipboard'ı dosyaya kaydet
cb -o cikti.txt

# Sürüm bilgisi
cb -v
```

## Platform Gereksinimleri

| Platform | Araç | Notlar |
|----------|------|--------|
| macOS | `pbcopy` / `pbpaste` | Zaten kurulu |
| Linux (X11) | `xclip` veya `xsel` | `sudo apt install xclip` |
| Linux (Wayland) | `wl-copy` / `wl-paste` | `sudo apt install wl-clipboard` |
| Windows | `clip.exe` / PowerShell | Zaten kurulu |

## Lisans

MIT