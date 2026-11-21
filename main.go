// Discord GIF Presence: a tray app that shows a GIF, "???" and a line of
// random glyphs as your Discord status. While Valorant is open (Windows only)
// it shows Playing / In Queue / In Lobby and the game mode instead.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/systray"
)

//go:embed icon.ico
var icon []byte

const interval = 15 * time.Second // Discord rate limit

type Config struct {
	ClientID     string `json:"client_id"`
	Details      string `json:"details"`
	Image        string `json:"large_image"`
	ImageText    string `json:"large_text"`
	GlyphMin     int    `json:"glyph_min"`
	GlyphMax     int    `json:"glyph_max"`
	Valorant     bool   `json:"valorant"`
	ValorantIcon string `json:"valorant_icon"`
}

var defaults = Config{
	ClientID:     "1544237311099797504",
	Details:      "???",
	Image:        "https://i.imgur.com/mdk7QJh.gif",
	ImageText:    "رَمْز",
	GlyphMin:     6,
	GlyphMax:     10,
	Valorant:     true,
	ValorantIcon: "valorant",
}

// Glyphs from several scripts, chosen for broad font support on Windows,
// macOS and mobile Discord. No combining marks, no Quranic symbols.
var glyphs = []rune("ⵣⵔⵙⵜⵀⵎⵏⵓⵕⵖ" + "ᚱᛟᚦᛉᚷᛞᚨᛗᛋᚹ" + "ΨΩΔλΣξΦπθ" + "ՖՃՁՋՊՔ" + "ლჯქჭწღ" +
	"ሀሐጸፀቐዘ" + "ᏜᏝᏡᏪᏫᏰ" + "زرمعقشطظ" + "∴∆⌬⊕∞⌘◊※")

var (
	dir         = configDir()
	configPath  = filepath.Join(dir, "config.json")
	profilesDir = filepath.Join(dir, "profiles")
	paused      atomic.Bool
	wake        = make(chan struct{}, 1)
	status      *systray.MenuItem
	lock        net.Listener // held for the app's lifetime: one copy at a time
)

func configDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, "discord-gif-presence")
}

func loadConfig() (Config, error) {
	c := defaults
	b, err := os.ReadFile(configPath)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

func saveConfig(c Config) error {
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(configPath, b, 0o644)
}

func poke() {
	select {
	case wake <- struct{}{}:
	default:
	}
}

func main() {
	os.MkdirAll(dir, 0o755)
	if f, err := os.Create(filepath.Join(dir, "log.txt")); err == nil {
		log.SetOutput(f)
	}
	var err error
	if lock, err = net.Listen("tcp", "127.0.0.1:47321"); err != nil {
		log.Print("already running")
		return
	}
	// First run: write the settings file and start at login by default.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		saveConfig(defaults)
		if err := setAutostart(true); err != nil {
			log.Print("autostart: ", err)
		}
	}
	systray.Run(onReady, nil)
}

func onReady() {
	systray.SetIcon(icon)
	systray.SetTooltip("Discord GIF Presence")
	status = systray.AddMenuItem("Starting...", "")
	status.Disable()
	systray.AddSeparator()
	fromLink := systray.AddMenuItem("Image from link...", "Paste a link to a GIF or image")
	fromFile := systray.AddMenuItem("Image from file...", "Upload a GIF or image from this computer")
	settings := systray.AddMenuItem("Open configuration file", "Edit the text, glyphs and more")
	profiles := systray.AddMenuItem("Profiles", "Save and switch between setups")
	saveProfile := profiles.AddSubMenuItem("Save current as new profile...", "")
	profiles.AddSeparator()
	found, _ := filepath.Glob(filepath.Join(profilesDir, "*.json"))
	for _, f := range found {
		addProfile(profiles, strings.TrimSuffix(filepath.Base(f), ".json"))
	}
	systray.AddSeparator()
	boot := systray.AddMenuItemCheckbox("Start at login", "", autostartEnabled())
	pause := systray.AddMenuItemCheckbox("Pause", "Hide the status from Discord", false)
	quit := systray.AddMenuItem("Quit", "")

	go run()
	for {
		select {
		case <-fromLink.ClickedCh:
			changeImage(askLink)
		case <-fromFile.ClickedCh:
			changeImage(askFile)
		case <-settings.ClickedCh:
			openFile(configPath)
		case <-saveProfile.ClickedCh:
			if name, isNew := saveAsProfile(); isNew {
				addProfile(profiles, name)
			}
		case <-boot.ClickedCh:
			if err := setAutostart(!boot.Checked()); err != nil {
				log.Print("autostart: ", err)
			} else if boot.Checked() {
				boot.Uncheck()
			} else {
				boot.Check()
			}
		case <-pause.ClickedCh:
			if pause.Checked() {
				pause.Uncheck()
			} else {
				pause.Check()
			}
			paused.Store(pause.Checked())
			poke()
		case <-quit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

// changeImage asks for a new image link and makes it the large image.
// An empty link means the user cancelled.
func changeImage(ask func(current string) string) {
	c, err := loadConfig()
	if err != nil {
		log.Print("not changing image, configuration file is broken: ", err)
		openFile(configPath)
		return
	}
	link := ask(c.Image)
	if link == "" || link == c.Image {
		return
	}
	c.Image = link
	if err := saveConfig(c); err != nil {
		showMessage("Could not save the configuration file: " + err.Error())
		return
	}
	poke()
}

func askLink(current string) string {
	prompt := "Paste a link to a GIF or image (it must start with https://):"
	for {
		link := strings.TrimSpace(askText(prompt, current))
		if link == "" || strings.HasPrefix(link, "https://") && !strings.ContainsAny(link, " \t") {
			return link
		}
		prompt = "That link does not start with https://. Try again:"
	}
}

func askFile(string) string {
	path := pickFile()
	if path == "" {
		return ""
	}
	link, err := uploadImage(path)
	if err != nil {
		log.Print("upload: ", err)
		showMessage("Could not use this image. " + err.Error())
		return ""
	}
	log.Printf("uploaded %s to %s", path, link)
	return link
}

// A profile is a copy of config.json in the profiles folder. Using one copies
// it back over config.json, so config.json is always the active setup.
func addProfile(parent *systray.MenuItem, name string) {
	item := parent.AddSubMenuItem(name, "Use this profile")
	go func() {
		for range item.ClickedCh {
			b, err := os.ReadFile(filepath.Join(profilesDir, name+".json"))
			var c Config
			if err == nil {
				err = json.Unmarshal(b, &c)
			}
			if err == nil {
				err = os.WriteFile(configPath, b, 0o644)
			}
			if err != nil {
				log.Printf("profile %s: %v", name, err)
				continue
			}
			log.Print("using profile ", name)
			poke()
		}
	}()
}

func saveAsProfile() (name string, isNew bool) {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) {
			return -1 // not allowed in Windows file names
		}
		return r
	}, askText("Name for this profile:", "")))
	b, err := os.ReadFile(configPath)
	if name == "" || err != nil {
		return "", false
	}
	p := filepath.Join(profilesDir, name+".json")
	_, err = os.Stat(p)
	isNew = os.IsNotExist(err)
	os.MkdirAll(profilesDir, 0o755)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		log.Print("save profile: ", err)
		return "", false
	}
	return name, isNew
}

func randomGlyphs(min, max int) string {
	if max < min {
		max = min
	}
	n := min + rand.Intn(max-min+1)
	out := make([]string, n)
	for i := range out {
		out[i] = string(glyphs[rand.Intn(len(glyphs))])
	}
	return strings.Join(out, " ")
}

var lastStatus string

func setStatus(s string) {
	if s != lastStatus {
		lastStatus = s
		status.SetTitle(s)
		log.Print(s)
	}
}

func run() {
	var (
		d        *discord
		cfg      = defaults
		cfgErr   string
		val      valorant
		valState string
		valStart int64
		last     map[string]any
		started  = time.Now().Unix()
	)
	for {
		if c, err := loadConfig(); err == nil {
			cfg, cfgErr = c, ""
		} else if err.Error() != cfgErr {
			cfgErr = err.Error()
			log.Print("configuration file: ", err)
		}

		if d != nil && (paused.Load() || d.clientID != cfg.ClientID) {
			d.Close() // Discord clears the status when we disconnect
			d = nil
		}

		if paused.Load() {
			setStatus("Paused")
		} else {
			if d == nil {
				var err error
				if d, err = dial(cfg.ClientID); err != nil {
					setStatus("Waiting for Discord...")
					log.Print(err)
				} else {
					last = nil
				}
			}
			if d != nil {
				a := map[string]any{
					"details":    cfg.Details,
					"state":      randomGlyphs(cfg.GlyphMin, cfg.GlyphMax),
					"timestamps": map[string]any{"start": started},
					"assets":     map[string]any{"large_image": cfg.Image, "large_text": cfg.ImageText},
				}
				s := ""
				if cfg.Valorant {
					s = val.state()
				}
				if s != valState {
					valState, valStart = s, time.Now().Unix() // timer restarts on each Valorant state
					log.Printf("valorant: %q", s)
				}
				if s != "" {
					a["details"] = "VALORANT"
					a["state"] = s
					a["timestamps"] = map[string]any{"start": valStart}
					a["assets"] = map[string]any{
						"large_image": cfg.Image, "large_text": cfg.ImageText,
						"small_image": cfg.ValorantIcon, "small_text": "VALORANT",
					}
				}
				// Valorant payloads repeat between polls; do not resend them.
				if !reflect.DeepEqual(a, last) {
					if err := d.setActivity(a); err != nil {
						setStatus(fmt.Sprint("Discord error: ", err))
						d.Close()
						d = nil
					} else {
						last = a
						setStatus("Showing on Discord")
					}
				}
			}
		}

		select {
		case <-time.After(interval):
		case <-wake:
		}
	}
}
