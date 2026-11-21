package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	runName = "DiscordGifPresence"
)

func ipcPaths() (p []string) {
	for i := 0; i < 10; i++ {
		p = append(p, fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i))
	}
	return p
}

func dialPipe(p string) (io.ReadWriteCloser, error) { return os.OpenFile(p, os.O_RDWR, 0) }

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runName)
	return err == nil
}

func setAutostart(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		return k.DeleteValue(runName)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(runName, `"`+exe+`"`)
}

// powershell runs a script that shows a built-in Windows dialog. Text goes
// in through environment variables so a link or file name can never break
// out of the script, and comes out as UTF-8.
func powershell(script, prompt, def string) string {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8; "+script)
	cmd.Env = append(os.Environ(), "ASK_PROMPT="+prompt, "ASK_DEFAULT="+def)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW: no console flash
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func askText(prompt, def string) string {
	return powershell(`Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.Interaction]::InputBox($env:ASK_PROMPT, 'Discord GIF Presence', $env:ASK_DEFAULT)`, prompt, def)
}

func pickFile() string {
	return powershell(`Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.OpenFileDialog; $d.Filter = 'Images|*.png;*.jpg;*.jpeg;*.gif;*.webp'; if ($d.ShowDialog() -eq 'OK') { $d.FileName }`, "", "")
}

func showMessage(msg string) {
	powershell(`Add-Type -AssemblyName System.Windows.Forms; [void][System.Windows.Forms.MessageBox]::Show($env:ASK_PROMPT, 'Discord GIF Presence')`, msg, "")
}

func openFile(p string) { exec.Command("notepad", p).Start() }
