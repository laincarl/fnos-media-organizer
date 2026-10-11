package organizer

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	Rules Rules `json:"rules"`
}

func loadSettings(path string) Settings {
	result := Settings{Rules: defaultRules()}
	if b, e := os.ReadFile(path); e == nil {
		var s Settings
		if json.Unmarshal(b, &s) == nil {
			if s.Rules.EpisodeFilename == "{title} - S{season}E{episode}" {
				s.Rules.EpisodeFilename = defaultRules().EpisodeFilename
			}
			s.Rules = normalizedRules(s.Rules)
			if e = validateRules(s.Rules); e == nil {
				result = s
			}
		}
	}
	return result
}
func saveSettings(path string, s Settings) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e != nil {
		f.Close()
		return e
	}
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(name, path); e != nil {
		return e
	}
	syncDir(filepath.Dir(path))
	return nil
}
