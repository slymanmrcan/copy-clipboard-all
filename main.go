package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const version = "1.1.0"

const usage = `cb - clipboard CLI tool

KULLANIM:
  cb [seçenekler] [metin veya dosya]

ÖRNEKLER:
  cb "merhaba dünya"         # metni kopyala
  cb dosya.txt               # dosya içeriğini kopyala
  echo "hello" | cb          # pipe çıktısını kopyala
  ls -la | cb                # komut çıktısını kopyala
  cb -p                      # clipboard içeriğini ekrana yaz (paste)
  cb -o dosya.txt            # clipboard içeriğini dosyaya yaz
  cb --osc52                 # sadece OSC 52 kullan (SSH için)

SEÇENEKLER:
  -p, --paste      Clipboard içeriğini stdout'a yaz
  -o <dosya>       Clipboard içeriğini dosyaya kaydet
  --osc52          OSC 52 escape sequence ile kopyala (SSH/headless)
  -v, --version    Sürüm bilgisini göster
  -h, --help       Bu yardım mesajını göster

NOTLAR:
  Öncelik sırası (otomatik tespit):
    1. macOS       → pbcopy/pbpaste
    2. Linux X11   → xclip veya xsel
    3. Linux Wayland → wl-copy/wl-paste
    4. SSH / headless → OSC 52 (terminal üzerinden)
    5. Windows     → clip.exe

  OSC 52: SSH üzerinden çalışır. Terminal desteği:
    iTerm2, Kitty, Alacritty, WezTerm, Windows Terminal, GNOME Terminal (yeni)
    tmux için: set -g set-clipboard on  (tmux.conf)
`

func main() {
	if len(os.Args) < 2 {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fatalf("stdin okunamadı: %v", err)
			}
			copyToClipboard(data, false)
			fmt.Fprintf(os.Stderr, "✓ %d karakter kopyalandı\n", len(data))
			return
		}
		fmt.Print(usage)
		os.Exit(0)
	}

	forceOSC52 := false
	args := os.Args[1:]

	// --osc52 flag'ini filtrele
	filtered := args[:0]
	for _, a := range args {
		if a == "--osc52" {
			forceOSC52 = true
		} else {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	if len(args) == 0 {
		// sadece --osc52 verilmişse stdin'i bekle
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fatalf("stdin okunamadı: %v", err)
			}
			copyToClipboard(data, forceOSC52)
			fmt.Fprintf(os.Stderr, "✓ %d karakter kopyalandı\n", len(data))
		} else {
			fmt.Print(usage)
		}
		return
	}

	arg := args[0]

	switch arg {
	case "-h", "--help":
		fmt.Print(usage)
	case "-v", "--version":
		fmt.Printf("cb v%s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	case "-p", "--paste":
		data := pasteFromClipboard()
		fmt.Print(string(data))
	case "-o", "--output":
		if len(args) < 2 {
			fatalf("-o seçeneği için dosya yolu gerekli")
		}
		data := pasteFromClipboard()
		if err := os.WriteFile(args[1], data, 0644); err != nil {
			fatalf("dosya yazılamadı: %v", err)
		}
		fmt.Fprintf(os.Stderr, "✓ %d karakter '%s' dosyasına yazıldı\n", len(data), args[1])
	default:
		if fileExists(arg) {
			data, err := os.ReadFile(arg)
			if err != nil {
				fatalf("dosya okunamadı: %v", err)
			}
			copyToClipboard(data, forceOSC52)
			fmt.Fprintf(os.Stderr, "✓ '%s' kopyalandı (%d karakter)\n", arg, len(data))
		} else {
			text := strings.Join(args, " ")
			copyToClipboard([]byte(text), forceOSC52)
			fmt.Fprintf(os.Stderr, "✓ '%s' kopyalandı\n", truncate(text, 50))
		}
	}
}

// ---------- clipboard ----------

func copyToClipboard(data []byte, forceOSC52 bool) {
	if forceOSC52 {
		osc52Copy(data)
		return
	}

	switch runtime.GOOS {
	case "darwin":
		if tryCmd("pbcopy", nil, data) {
			return
		}
	case "windows":
		if tryCmd("clip", nil, data) {
			return
		}
	default:
		// Wayland
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if tryCmd("wl-copy", nil, data) {
				return
			}
		}
		// X11
		if os.Getenv("DISPLAY") != "" {
			if tryCmd("xclip", []string{"-selection", "clipboard"}, data) {
				return
			}
			if tryCmd("xsel", []string{"--clipboard", "--input"}, data) {
				return
			}
		}
	}

	// Fallback: OSC 52 — SSH veya headless ortam
	fmt.Fprintf(os.Stderr, "ℹ  native clipboard bulunamadı, OSC 52 deneniyor...\n")
	osc52Copy(data)
}

func pasteFromClipboard() []byte {
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("pbpaste").Output(); err == nil {
			return out
		}
	case "windows":
		if out, err := exec.Command("powershell", "-command", "Get-Clipboard").Output(); err == nil {
			return out
		}
	default:
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if out, err := exec.Command("wl-paste", "--no-newline").Output(); err == nil {
				return out
			}
		}
		if os.Getenv("DISPLAY") != "" {
			if out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output(); err == nil {
				return out
			}
			if out, err := exec.Command("xsel", "--clipboard", "--output").Output(); err == nil {
				return out
			}
		}
	}
	fatalf("clipboard'dan okuma başarısız.\nOSC 52 sadece yazma destekler, okuma için xclip/xsel kurun.")
	return nil
}

// OSC 52 — terminal escape sequence ile clipboard'a yaz
// SSH üzerinden bile çalışır (terminal destekliyorsa)
func osc52Copy(data []byte) {
	encoded := base64.StdEncoding.EncodeToString(data)

	// /dev/tty'ye yaz — stdout/stderr pipe'da olsa bile çalışır
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		// /dev/tty yoksa (Windows/bazı CI) stdout'a yaz
		fmt.Printf("\033]52;c;%s\a", encoded)
		return
	}
	defer tty.Close()

	// tmux içindeyse wrap et
	if os.Getenv("TMUX") != "" {
		fmt.Fprintf(tty, "\033Ptmux;\033\033]52;c;%s\a\033\\", encoded)
	} else {
		fmt.Fprintf(tty, "\033]52;c;%s\a", encoded)
	}
}

// ---------- yardımcılar ----------

func tryCmd(name string, args []string, stdin []byte) bool {
	path, err := exec.LookPath(name)
	if err != nil {
		return false
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	return cmd.Run() == nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "hata: "+format+"\n", args...)
	os.Exit(1)
}
