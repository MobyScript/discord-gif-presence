//go:build !windows

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func ipcPaths() (p []string) {
	for _, d := range []string{os.Getenv("XDG_RUNTIME_DIR"), os.Getenv("TMPDIR"), os.Getenv("TMP"), os.Getenv("TEMP"), "/tmp"} {
		for i := 0; d != "" && i < 10; i++ {
			p = append(p, filepath.Join(d, fmt.Sprintf("discord-ipc-%d", i)))
		}
	}
	// macOS login items may start without TMPDIR set.
	found, _ := filepath.Glob("/var/folders/*/*/T/discord-ipc-*")
	return append(p, found...)
}

func dialPipe(p string) (io.ReadWriteCloser, error) { return net.Dial("unix", p) }

// autostartFile returns the macOS LaunchAgent or Linux autostart entry.
func autostartFile() (path, body string) {
	home, _ := os.UserHomeDir()
	exe, _ := os.Executable()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "LaunchAgents", "com.mobyscript.discord-gif-presence.plist"),
			`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.mobyscript.discord-gif-presence</string>
<key>ProgramArguments</key><array><string>` + exe + `</string></array>
<key>RunAtLoad</key><true/>
</dict></plist>
`
	}
	return filepath.Join(home, ".config", "autostart", "discord-gif-presence.desktop"),
		"[Desktop Entry]\nType=Application\nName=Discord GIF Presence\nExec=\"" + exe + "\"\n"
}

func autostartEnabled() bool {
	p, _ := autostartFile()
	_, err := os.Stat(p)
	return err == nil
}

func setAutostart(on bool) error {
	p, body := autostartFile()
	if !on {
		return os.Remove(p)
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	return os.WriteFile(p, []byte(body), 0o644)
}

// dialog shows a native dialog: an AppleScript on macOS, zenity elsewhere.
// Text goes in through environment variables so a link can never break out
// of the script.
func dialog(appleScript string, zenityArgs []string, prompt, def string) string {
	cmd := exec.Command("zenity", append([]string{"--title=Discord GIF Presence"}, zenityArgs...)...)
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("osascript", "-e", appleScript)
	}
	cmd.Env = append(os.Environ(), "ASK_PROMPT="+prompt, "ASK_DEFAULT="+def)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func askText(prompt, def string) string {
	return dialog(`text returned of (display dialog (system attribute "ASK_PROMPT") default answer (system attribute "ASK_DEFAULT") with title "Discord GIF Presence")`,
		[]string{"--entry", "--text=" + prompt, "--entry-text=" + def}, prompt, def)
}

func pickFile() string {
	return dialog(`POSIX path of (choose file of type {"public.image"})`,
		[]string{"--file-selection", "--file-filter=*.png *.jpg *.jpeg *.gif *.webp"}, "", "")
}

func showMessage(msg string) {
	dialog(`display alert "Discord GIF Presence" message (system attribute "ASK_PROMPT")`,
		[]string{"--error", "--text=" + msg}, msg, "")
}

func openFile(p string) {
	if runtime.GOOS == "darwin" {
		exec.Command("open", "-t", p).Start()
	} else {
		exec.Command("xdg-open", p).Start()
	}
}
