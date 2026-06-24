package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const version = "1.2.1"

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

  OSC 52 yalnızca bağlı ve desteklenen bir terminal varken çalışır.
  Terminal desteği:
    iTerm2, Kitty, Alacritty, WezTerm, Windows Terminal, GNOME Terminal (yeni)
    tmux için: set -g set-clipboard on  (tmux.conf)
`

type cli struct {
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	stdinStat     func() (os.FileInfo, error)
	goos          string
	getenv        func(string) string
	stat          func(string) (os.FileInfo, error)
	readFile      func(string) ([]byte, error)
	writeFile     func(string, []byte) error
	lookPath      func(string) (string, error)
	runCommand    func(string, []string, []byte) error
	commandOutput func(string, []string) ([]byte, error)
	openTTY       func() (io.WriteCloser, error)
}

func newCLI() *cli {
	return &cli{
		stdin:     os.Stdin,
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		stdinStat: os.Stdin.Stat,
		goos:      runtime.GOOS,
		getenv:    os.Getenv,
		stat:      os.Stat,
		readFile:  os.ReadFile,
		writeFile: writePrivateFile,
		lookPath:  exec.LookPath,
		runCommand: func(name string, args []string, stdin []byte) error {
			// #nosec G204 -- name is resolved from a fixed internal clipboard command list.
			cmd := exec.Command(name, args...)
			cmd.Stdin = bytes.NewReader(stdin)
			return cmd.Run()
		},
		commandOutput: func(name string, args []string) ([]byte, error) {
			// #nosec G204 -- name and args come from a fixed internal clipboard command list.
			return exec.Command(name, args...).Output()
		},
		openTTY: func() (io.WriteCloser, error) {
			return os.OpenFile(terminalPath(runtime.GOOS), os.O_WRONLY, 0)
		},
	}
}

func main() {
	os.Exit(runMain(os.Args[1:], newCLI()))
}

func runMain(args []string, app *cli) int {
	if err := app.run(args); err != nil {
		_, _ = fmt.Fprintf(app.stderr, "hata: %v\n", err)
		return 1
	}
	return 0
}

func (c *cli) run(args []string) error {
	if len(args) == 0 {
		hasInput, err := c.hasPipedInput()
		if err != nil {
			return err
		}
		if !hasInput {
			_, err = fmt.Fprint(c.stdout, usage)
			return err
		}
		return c.copyStdin(false)
	}

	forceOSC52 := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--osc52" {
			forceOSC52 = true
			continue
		}
		filtered = append(filtered, arg)
	}
	args = filtered

	if len(args) == 0 {
		hasInput, err := c.hasPipedInput()
		if err != nil {
			return err
		}
		if !hasInput {
			_, err = fmt.Fprint(c.stdout, usage)
			return err
		}
		return c.copyStdin(forceOSC52)
	}

	arg := args[0]
	switch arg {
	case "-h", "--help":
		_, err := fmt.Fprint(c.stdout, usage)
		return err
	case "-v", "--version":
		_, err := fmt.Fprintf(c.stdout, "cb v%s (%s/%s)\n", version, c.goos, runtime.GOARCH)
		return err
	case "-p", "--paste":
		data, err := c.pasteFromClipboard()
		if err != nil {
			return err
		}
		_, err = c.stdout.Write(data)
		return err
	case "-o", "--output":
		if len(args) < 2 {
			return errors.New("-o seçeneği için dosya yolu gerekli")
		}
		data, err := c.pasteFromClipboard()
		if err != nil {
			return err
		}
		if err := c.writeFile(args[1], data); err != nil {
			return fmt.Errorf("dosya yazılamadı: %w", err)
		}
		_, err = fmt.Fprintf(c.stderr, "✓ %d karakter '%s' dosyasına yazıldı\n", len(data), args[1])
		return err
	default:
		exists, err := c.fileExists(arg)
		if err != nil {
			return fmt.Errorf("dosya kontrol edilemedi: %w", err)
		}
		if exists {
			data, err := c.readFile(arg)
			if err != nil {
				return fmt.Errorf("dosya okunamadı: %w", err)
			}
			if err := c.copyToClipboard(data, forceOSC52); err != nil {
				return err
			}
			_, err = fmt.Fprintf(c.stderr, "✓ '%s' kopyalandı (%d karakter)\n", arg, len(data))
			return err
		}

		text := strings.Join(args, " ")
		if err := c.copyToClipboard([]byte(text), forceOSC52); err != nil {
			return err
		}
		_, err = fmt.Fprintf(c.stderr, "✓ '%s' kopyalandı\n", truncate(text, 50))
		return err
	}
}

func (c *cli) hasPipedInput() (bool, error) {
	stat, err := c.stdinStat()
	if err != nil {
		return false, fmt.Errorf("stdin durumu okunamadı: %w", err)
	}
	return stat.Mode()&os.ModeCharDevice == 0, nil
}

func (c *cli) copyStdin(forceOSC52 bool) error {
	data, err := io.ReadAll(c.stdin)
	if err != nil {
		return fmt.Errorf("stdin okunamadı: %w", err)
	}
	if err := c.copyToClipboard(data, forceOSC52); err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.stderr, "✓ %d karakter kopyalandı\n", len(data))
	return err
}

func (c *cli) copyToClipboard(data []byte, forceOSC52 bool) error {
	if forceOSC52 {
		return c.osc52Copy(data)
	}

	switch c.goos {
	case "darwin":
		if c.tryCmd("pbcopy", nil, data) {
			return nil
		}
	case "windows":
		if c.tryCmd("clip", nil, data) {
			return nil
		}
	default:
		if c.getenv("WAYLAND_DISPLAY") != "" && c.tryCmd("wl-copy", nil, data) {
			return nil
		}
		if c.getenv("DISPLAY") != "" {
			if c.tryCmd("xclip", []string{"-selection", "clipboard"}, data) {
				return nil
			}
			if c.tryCmd("xsel", []string{"--clipboard", "--input"}, data) {
				return nil
			}
		}
	}

	if _, err := fmt.Fprintln(c.stderr, "ℹ  native clipboard bulunamadı, OSC 52 deneniyor..."); err != nil {
		return err
	}
	return c.osc52Copy(data)
}

func (c *cli) pasteFromClipboard() ([]byte, error) {
	switch c.goos {
	case "darwin":
		if out, err := c.commandOutput("pbpaste", nil); err == nil {
			return out, nil
		}
	case "windows":
		if out, err := c.commandOutput("powershell", []string{"-command", "Get-Clipboard"}); err == nil {
			return out, nil
		}
	default:
		if c.getenv("WAYLAND_DISPLAY") != "" {
			if out, err := c.commandOutput("wl-paste", []string{"--no-newline"}); err == nil {
				return out, nil
			}
		}
		if c.getenv("DISPLAY") != "" {
			if out, err := c.commandOutput("xclip", []string{"-selection", "clipboard", "-o"}); err == nil {
				return out, nil
			}
			if out, err := c.commandOutput("xsel", []string{"--clipboard", "--output"}); err == nil {
				return out, nil
			}
		}
	}
	return nil, errors.New("clipboard'dan okuma başarısız; OSC 52 yalnızca yazmayı destekler")
}

func (c *cli) osc52Copy(data []byte) error {
	tty, err := c.openTTY()
	if err != nil {
		return fmt.Errorf("OSC 52 için bağlı terminal bulunamadı: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	var sequence string
	if c.getenv("TMUX") != "" {
		sequence = fmt.Sprintf("\033Ptmux;\033\033]52;c;%s\a\033\\", encoded)
	} else {
		sequence = fmt.Sprintf("\033]52;c;%s\a", encoded)
	}

	if _, err := io.WriteString(tty, sequence); err != nil {
		_ = tty.Close()
		return fmt.Errorf("OSC 52 terminale yazılamadı: %w", err)
	}
	if err := tty.Close(); err != nil {
		return fmt.Errorf("terminal kapatılamadı: %w", err)
	}
	return nil
}

func (c *cli) tryCmd(name string, args []string, stdin []byte) bool {
	path, err := c.lookPath(name)
	if err != nil {
		return false
	}
	return c.runCommand(path, args, stdin) == nil
}

func (c *cli) fileExists(path string) (bool, error) {
	info, err := c.stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

func writePrivateFile(path string, data []byte) error {
	// #nosec G304,G703 -- the output path is explicitly selected by the CLI user.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func terminalPath(goos string) string {
	if goos == "windows" {
		return "CONOUT$"
	}
	return "/dev/tty"
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
