package auth

import (
	"os"
	"path/filepath"
)

func projectFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "scrum")
	os.MkdirAll(path, 0700)
	return filepath.Join(path, "project"), nil
}

func SaveProject(key string) error {
	f, err := projectFile()
	if err != nil {
		return err
	}
	return os.WriteFile(f, []byte(key), 0600)
}

func LoadProject() (string, error) {
	f, err := projectFile()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func DeleteProject() error {
	f, err := projectFile()
	if err != nil {
		return err
	}
	return os.Remove(f)
}