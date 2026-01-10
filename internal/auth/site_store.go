package auth

import (
    "os"
    "path/filepath"
)

func siteFile() (string, error) {
    dir, err := os.UserConfigDir()
    if err != nil {
        return "", err
    }
    path := filepath.Join(dir, "scrum")
    os.MkdirAll(path, 0700)
    return filepath.Join(path, "site"), nil
}

func SaveSite(cloudID string) error {
    f, err := siteFile()
    if err != nil {
        return err
    }
    return os.WriteFile(f, []byte(cloudID), 0600)
}

func LoadSite() (string, error) {
    f, err := siteFile()
    if err != nil {
        return "", err
    }
    b, err := os.ReadFile(f)
    if err != nil {
        return "", err
    }
    return string(b), nil
}
