package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Profile struct {
	Name     string `json:"Name"`
	Target   string `json:"Target"`
	Port     int    `json:"Port,omitempty"`
	Identity string `json:"Identity,omitempty"`
}

func GetStorePath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "connections.json"
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "TerminalLibghosty", "connections.json")
}

func LoadProfiles() []Profile {
	store := GetStorePath()
	data, err := os.ReadFile(store)
	if err != nil {
		return []Profile{}
	}
	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return []Profile{}
	}
	return profiles
}

func SaveProfiles(profiles []Profile) error {
	store := GetStorePath()
	dir := filepath.Dir(store)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(store, data, 0644)
}
