package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeFileInfo struct {
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return "stdin" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

type bufferCloser struct {
	bytes.Buffer
	closeErr error
}

func (b *bufferCloser) Close() error { return b.closeErr }

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

func testCLI() (*cli, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &cli{
		stdin:     strings.NewReader(""),
		stdout:    stdout,
		stderr:    stderr,
		stdinStat: func() (os.FileInfo, error) { return fakeFileInfo{mode: os.ModeCharDevice}, nil },
		goos:      "linux",
		getenv:    func(string) string { return "" },
		stat:      os.Stat,
		readFile:  os.ReadFile,
		writeFile: writePrivateFile,
		lookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		runCommand: func(string, []string, []byte) error {
			return errors.New("unexpected command")
		},
		commandOutput: func(string, []string) ([]byte, error) {
			return nil, errors.New("clipboard unavailable")
		},
		openTTY: func() (io.WriteCloser, error) {
			return nil, errors.New("no tty")
		},
	}
	return app, stdout, stderr
}

func TestRunHelpAndVersion(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains string
	}{
		{name: "no arguments", contains: "KULLANIM:"},
		{name: "short help", args: []string{"-h"}, contains: "OSC 52"},
		{name: "long help", args: []string{"--help"}, contains: "SEÇENEKLER:"},
		{name: "short version", args: []string{"-v"}, contains: "cb v" + version + " (linux/"},
		{name: "long version", args: []string{"--version"}, contains: "cb v" + version + " (linux/"},
		{name: "osc52 without stdin", args: []string{"--osc52"}, contains: "KULLANIM:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, stdout, _ := testCLI()
			if err := app.run(tt.args); err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if !strings.Contains(stdout.String(), tt.contains) {
				t.Fatalf("stdout = %q, expected to contain %q", stdout.String(), tt.contains)
			}
		})
	}
}

func TestRunCopiesTextWithX11(t *testing.T) {
	app, _, stderr := testCLI()
	app.getenv = func(key string) string {
		if key == "DISPLAY" {
			return ":1"
		}
		return ""
	}
	app.lookPath = func(name string) (string, error) {
		if name == "xclip" {
			return "/usr/bin/xclip", nil
		}
		return "", os.ErrNotExist
	}

	var gotName string
	var gotArgs []string
	var gotInput []byte
	app.runCommand = func(name string, args []string, input []byte) error {
		gotName = name
		gotArgs = args
		gotInput = append([]byte(nil), input...)
		return nil
	}

	if err := app.run([]string{"merhaba", "dünya"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if gotName != "/usr/bin/xclip" {
		t.Fatalf("command = %q", gotName)
	}
	if strings.Join(gotArgs, " ") != "-selection clipboard" {
		t.Fatalf("args = %v", gotArgs)
	}
	if string(gotInput) != "merhaba dünya" {
		t.Fatalf("input = %q", gotInput)
	}
	if !strings.Contains(stderr.String(), "kopyalandı") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCopiesFileWithForcedOSC52(t *testing.T) {
	app, _, stderr := testCLI()
	inputPath := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(inputPath, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	tty := &bufferCloser{}
	app.openTTY = func() (io.WriteCloser, error) { return tty, nil }

	if err := app.run([]string{"--osc52", inputPath}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if tty.String() != "\033]52;c;c2VjcmV0\a" {
		t.Fatalf("OSC52 sequence = %q", tty.String())
	}
	if !strings.Contains(stderr.String(), inputPath) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCopiesPipedStdin(t *testing.T) {
	app, _, stderr := testCLI()
	app.stdin = strings.NewReader("pipe data")
	app.stdinStat = func() (os.FileInfo, error) { return fakeFileInfo{}, nil }
	tty := &bufferCloser{}
	app.openTTY = func() (io.WriteCloser, error) { return tty, nil }

	if err := app.run([]string{"--osc52"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if tty.String() != "\033]52;c;cGlwZSBkYXRh\a" {
		t.Fatalf("OSC52 sequence = %q", tty.String())
	}
	if !strings.Contains(stderr.String(), "9 karakter") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPasteAndOutput(t *testing.T) {
	t.Run("paste to stdout", func(t *testing.T) {
		app, stdout, _ := testCLI()
		app.goos = "darwin"
		app.commandOutput = func(name string, args []string) ([]byte, error) {
			if name != "pbpaste" || len(args) != 0 {
				t.Fatalf("command = %q %v", name, args)
			}
			return []byte("clipboard"), nil
		}

		if err := app.run([]string{"--paste"}); err != nil {
			t.Fatalf("run() error = %v", err)
		}
		if stdout.String() != "clipboard" {
			t.Fatalf("stdout = %q", stdout.String())
		}
	})

	t.Run("output is always private", func(t *testing.T) {
		app, _, stderr := testCLI()
		app.goos = "darwin"
		app.commandOutput = func(string, []string) ([]byte, error) {
			return []byte("token"), nil
		}

		outputPath := filepath.Join(t.TempDir(), "clipboard.txt")
		if err := os.WriteFile(outputPath, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := app.run([]string{"-o", outputPath}); err != nil {
			t.Fatalf("run() error = %v", err)
		}

		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "token" {
			t.Fatalf("content = %q", data)
		}
		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatalf("mode = %o, expected 600", info.Mode().Perm())
		}
		if !strings.Contains(stderr.String(), "5 karakter") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})
}

func TestRunErrors(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*cli)
		args      []string
		contains  string
	}{
		{
			name:     "missing output path",
			args:     []string{"-o"},
			contains: "dosya yolu gerekli",
		},
		{
			name: "stdin stat error",
			configure: func(app *cli) {
				app.stdinStat = func() (os.FileInfo, error) { return nil, errors.New("stat failed") }
			},
			contains: "stdin durumu okunamadı",
		},
		{
			name: "stdin read error",
			configure: func(app *cli) {
				app.stdinStat = func() (os.FileInfo, error) { return fakeFileInfo{}, nil }
				app.stdin = errorReader{err: errors.New("read failed")}
			},
			contains: "stdin okunamadı",
		},
		{
			name: "file stat error",
			configure: func(app *cli) {
				app.stat = func(string) (os.FileInfo, error) { return nil, errors.New("permission denied") }
			},
			args:     []string{"file.txt"},
			contains: "dosya kontrol edilemedi",
		},
		{
			name: "file read error",
			configure: func(app *cli) {
				app.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }
				app.readFile = func(string) ([]byte, error) { return nil, errors.New("read denied") }
			},
			args:     []string{"file.txt"},
			contains: "dosya okunamadı",
		},
		{
			name:     "paste unavailable",
			args:     []string{"-p"},
			contains: "clipboard'dan okuma başarısız",
		},
		{
			name: "output write error",
			configure: func(app *cli) {
				app.goos = "darwin"
				app.commandOutput = func(string, []string) ([]byte, error) { return []byte("data"), nil }
				app.writeFile = func(string, []byte) error { return errors.New("disk full") }
			},
			args:     []string{"-o", "out.txt"},
			contains: "dosya yazılamadı",
		},
		{
			name:     "forced osc52 without terminal",
			args:     []string{"--osc52", "text"},
			contains: "bağlı terminal bulunamadı",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, _ := testCLI()
			if tt.configure != nil {
				tt.configure(app)
			}
			err := app.run(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("run() error = %v, expected %q", err, tt.contains)
			}
		})
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestClipboardFallbacks(t *testing.T) {
	t.Run("native command failure falls back to OSC52", func(t *testing.T) {
		app, _, stderr := testCLI()
		app.goos = "darwin"
		app.lookPath = func(string) (string, error) { return "/usr/bin/pbcopy", nil }
		app.runCommand = func(string, []string, []byte) error { return errors.New("pbcopy failed") }
		tty := &bufferCloser{}
		app.openTTY = func() (io.WriteCloser, error) { return tty, nil }

		if err := app.copyToClipboard([]byte("fallback"), false); err != nil {
			t.Fatalf("copyToClipboard() error = %v", err)
		}
		if !strings.Contains(stderr.String(), "OSC 52 deneniyor") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})

	t.Run("wayland is preferred", func(t *testing.T) {
		app, _, _ := testCLI()
		app.getenv = func(key string) string {
			if key == "WAYLAND_DISPLAY" || key == "DISPLAY" {
				return "set"
			}
			return ""
		}
		app.lookPath = func(name string) (string, error) { return name, nil }
		var command string
		app.runCommand = func(name string, _ []string, _ []byte) error {
			command = name
			return nil
		}

		if err := app.copyToClipboard([]byte("data"), false); err != nil {
			t.Fatalf("copyToClipboard() error = %v", err)
		}
		if command != "wl-copy" {
			t.Fatalf("command = %q", command)
		}
	})

	t.Run("xsel follows xclip failure", func(t *testing.T) {
		app, _, _ := testCLI()
		app.getenv = func(key string) string {
			if key == "DISPLAY" {
				return ":1"
			}
			return ""
		}
		app.lookPath = func(name string) (string, error) { return name, nil }
		var commands []string
		app.runCommand = func(name string, _ []string, _ []byte) error {
			commands = append(commands, name)
			if name == "xclip" {
				return errors.New("failed")
			}
			return nil
		}

		if err := app.copyToClipboard([]byte("data"), false); err != nil {
			t.Fatalf("copyToClipboard() error = %v", err)
		}
		if strings.Join(commands, ",") != "xclip,xsel" {
			t.Fatalf("commands = %v", commands)
		}
	})

	t.Run("windows clip", func(t *testing.T) {
		app, _, _ := testCLI()
		app.goos = "windows"
		app.lookPath = func(name string) (string, error) { return name, nil }
		app.runCommand = func(name string, _ []string, _ []byte) error {
			if name != "clip" {
				t.Fatalf("command = %q", name)
			}
			return nil
		}
		if err := app.copyToClipboard([]byte("data"), false); err != nil {
			t.Fatalf("copyToClipboard() error = %v", err)
		}
	})
}

func TestPasteFallbacks(t *testing.T) {
	tests := []struct {
		name      string
		goos      string
		env       map[string]string
		successOn string
	}{
		{name: "windows", goos: "windows", successOn: "powershell"},
		{name: "wayland", goos: "linux", env: map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, successOn: "wl-paste"},
		{name: "xclip", goos: "linux", env: map[string]string{"DISPLAY": ":1"}, successOn: "xclip"},
		{name: "xsel fallback", goos: "linux", env: map[string]string{"DISPLAY": ":1"}, successOn: "xsel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, _ := testCLI()
			app.goos = tt.goos
			app.getenv = func(key string) string { return tt.env[key] }
			app.commandOutput = func(name string, _ []string) ([]byte, error) {
				if name == tt.successOn {
					return []byte("clipboard"), nil
				}
				return nil, errors.New("failed")
			}

			data, err := app.pasteFromClipboard()
			if err != nil {
				t.Fatalf("pasteFromClipboard() error = %v", err)
			}
			if string(data) != "clipboard" {
				t.Fatalf("data = %q", data)
			}
		})
	}
}

func TestOSC52(t *testing.T) {
	t.Run("tmux sequence", func(t *testing.T) {
		app, _, _ := testCLI()
		app.getenv = func(key string) string {
			if key == "TMUX" {
				return "/tmp/tmux"
			}
			return ""
		}
		tty := &bufferCloser{}
		app.openTTY = func() (io.WriteCloser, error) { return tty, nil }

		if err := app.osc52Copy([]byte("abc")); err != nil {
			t.Fatalf("osc52Copy() error = %v", err)
		}
		if tty.String() != "\033Ptmux;\033\033]52;c;YWJj\a\033\\" {
			t.Fatalf("sequence = %q", tty.String())
		}
	})

	t.Run("write error", func(t *testing.T) {
		app, _, _ := testCLI()
		app.openTTY = func() (io.WriteCloser, error) {
			return writeCloser{Writer: errorWriter{err: errors.New("write failed")}}, nil
		}
		err := app.osc52Copy([]byte("abc"))
		if err == nil || !strings.Contains(err.Error(), "terminale yazılamadı") {
			t.Fatalf("osc52Copy() error = %v", err)
		}
	})

	t.Run("close error", func(t *testing.T) {
		app, _, _ := testCLI()
		app.openTTY = func() (io.WriteCloser, error) {
			return &bufferCloser{closeErr: errors.New("close failed")}, nil
		}
		err := app.osc52Copy([]byte("abc"))
		if err == nil || !strings.Contains(err.Error(), "terminal kapatılamadı") {
			t.Fatalf("osc52Copy() error = %v", err)
		}
	})
}

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }

func TestRunMain(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		app, _, _ := testCLI()
		if code := runMain([]string{"--help"}, app); code != 0 {
			t.Fatalf("code = %d", code)
		}
	})

	t.Run("error", func(t *testing.T) {
		app, _, stderr := testCLI()
		if code := runMain([]string{"--osc52", "text"}, app); code != 1 {
			t.Fatalf("code = %d", code)
		}
		if !strings.HasPrefix(stderr.String(), "hata:") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})
}

func TestNewCLI(t *testing.T) {
	app := newCLI()
	if app.stdin == nil || app.stdout == nil || app.stderr == nil {
		t.Fatal("standard streams must be configured")
	}
	if app.goos != runtime.GOOS {
		t.Fatalf("goos = %q, expected %q", app.goos, runtime.GOOS)
	}
}

func TestWritePrivateFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "out.txt")
	if err := writePrivateFile(path, []byte("data")); err == nil {
		t.Fatal("writePrivateFile() expected error")
	}
}

func TestFileExistsDirectory(t *testing.T) {
	app, _, _ := testCLI()
	exists, err := app.fileExists(t.TempDir())
	if err != nil {
		t.Fatalf("fileExists() error = %v", err)
	}
	if exists {
		t.Fatal("directory must not be treated as a file")
	}
}

func TestTerminalPath(t *testing.T) {
	if got := terminalPath("linux"); got != "/dev/tty" {
		t.Fatalf("linux terminal path = %q", got)
	}
	if got := terminalPath("windows"); got != "CONOUT$" {
		t.Fatalf("windows terminal path = %q", got)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		limit    int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"merhaba dünya", 9, "merhaba d..."},
		{"🚀🚀🚀🚀🚀", 3, "🚀🚀🚀..."},
	}

	for _, tt := range tests {
		if actual := truncate(tt.input, tt.limit); actual != tt.expected {
			t.Errorf("truncate(%q, %d) = %q; expected %q", tt.input, tt.limit, actual, tt.expected)
		}
	}
}
