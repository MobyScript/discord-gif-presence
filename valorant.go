package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// valorant reads ONLY your own presence from the local Riot Client.
// Every failure returns "" so the caller falls back to the glyph status.
type valorant struct{ port, auth, puuid string }

// The Riot Client serves a self-signed certificate on 127.0.0.1 only.
var riot = &http.Client{
	Timeout:   2 * time.Second,
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
}

var queueNames = map[string]string{
	"unrated": "Unrated", "competitive": "Competitive", "premier": "Premier",
	"swiftplay": "Swiftplay", "spikerush": "Spike Rush", "deathmatch": "Deathmatch",
	"hurm": "Team Deathmatch", "ggteam": "Escalation", "onefa": "Replication",
	"snowball": "Snowball Fight", "newmap": "New Map",
}

// state returns e.g. "Playing · Swiftplay", or "" when Valorant is not open.
func (v *valorant) state() string {
	if v.auth == "" && !v.connect() {
		return ""
	}
	if v.puuid == "" {
		var s struct{ Puuid string }
		if v.get("/chat/v1/session", &s) != nil || s.Puuid == "" {
			*v = valorant{}
			return ""
		}
		v.puuid = s.Puuid
	}
	var p struct {
		Presences []struct{ Puuid, Product, Private string }
	}
	if v.get("/chat/v4/presences", &p) != nil {
		*v = valorant{} // client closed or restarted: re-read the lockfile next time
		return ""
	}
	// The endpoint returns your whole friends list. Only use YOURS.
	for _, x := range p.Presences {
		if x.Puuid != v.puuid || x.Product != "valorant" {
			continue
		}
		var data map[string]any
		b, err := base64.StdEncoding.DecodeString(x.Private)
		if err != nil || json.Unmarshal(b, &data) != nil {
			return ""
		}
		return parsePresence(data)
	}
	return ""
}

func (v *valorant) connect() bool {
	if runtime.GOOS != "windows" {
		return false // Valorant is Windows-only
	}
	b, err := os.ReadFile(filepath.Join(os.Getenv("LOCALAPPDATA"), "Riot Games", "Riot Client", "Config", "lockfile"))
	parts := strings.Split(string(b), ":")
	if err != nil || len(parts) < 5 {
		return false
	}
	v.port = parts[2]
	v.auth = "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:"+parts[3]))
	return true
}

func (v *valorant) get(path string, out any) error {
	req, _ := http.NewRequest("GET", "https://127.0.0.1:"+v.port+path, nil)
	req.Header.Set("Authorization", v.auth)
	resp, err := riot.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("riot client: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// pick returns the first non-empty string at "section.field" or "field".
func pick(d map[string]any, paths ...string) string {
	for _, p := range paths {
		var v any
		if sec, field, ok := strings.Cut(p, "."); ok {
			m, _ := d[sec].(map[string]any)
			v = m[field]
		} else {
			v = d[p]
		}
		if s, _ := v.(string); s != "" {
			return s
		}
	}
	return ""
}

func parsePresence(d map[string]any) string {
	loop := pick(d, "matchPresenceData.sessionLoopState", "partyPresenceData.partyOwnerSessionLoopState", "sessionLoopState")
	party := pick(d, "partyPresenceData.partyState", "partyState")
	flow := pick(d, "matchPresenceData.provisioningFlow", "provisioningFlow")
	queue := strings.ToLower(pick(d, "matchPresenceData.queueId", "queueId"))

	mode := queueNames[queue]
	switch {
	case flow == "ShootingRange":
		mode = "The Range"
	case flow == "CustomGame" || party == "CUSTOM_GAME_SETUP":
		mode = "Custom"
	case mode != "":
	case queue != "":
		mode = strings.ToUpper(queue[:1]) + queue[1:]
	default:
		mode = "Unrated"
	}

	status := "In Lobby"
	if loop == "INGAME" || loop == "PREGAME" {
		status = "Playing"
	} else if party == "MATCHMAKING" || party == "MATCHMADE_GAME_STARTING" {
		status = "In Queue"
	}
	return status + " · " + mode
}
