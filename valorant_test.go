package main

import "testing"

func TestParsePresence(t *testing.T) {
	cases := []struct {
		data map[string]any
		want string
	}{
		{map[string]any{"matchPresenceData": map[string]any{"sessionLoopState": "INGAME", "queueId": "swiftplay"}}, "Playing · Swiftplay"},
		{map[string]any{"partyPresenceData": map[string]any{"partyState": "MATCHMAKING"}, "queueId": "competitive"}, "In Queue · Competitive"},
		{map[string]any{"sessionLoopState": "MENUS", "queueId": "Skirmish2v2"}, "In Lobby · Skirmish2v2"},
		{map[string]any{"matchPresenceData": map[string]any{"provisioningFlow": "ShootingRange"}}, "In Lobby · The Range"},
		{map[string]any{}, "In Lobby · Unrated"},
	}
	for _, c := range cases {
		if got := parsePresence(c.data); got != c.want {
			t.Errorf("parsePresence(%v) = %q, want %q", c.data, got, c.want)
		}
	}
}
