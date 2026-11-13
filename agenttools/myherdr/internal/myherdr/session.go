package myherdr

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

var (
	sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	paneIDPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]+:[A-Za-z0-9_-]+$`)
)

func ValidateSessionName(name string) error {
	if name == "." || name == ".." || !sessionNamePattern.MatchString(name) {
		return errors.New("session name must use 1 to 64 ASCII letters, digits, dots, underscores, or hyphens; . and .. are reserved")
	}
	return nil
}

func ValidatePaneID(id string) error {
	if !paneIDPattern.MatchString(id) {
		return errors.New("pane ID has an invalid format")
	}
	return nil
}

func SessionCommand(name string) string {
	return fmt.Sprintf("herdr --session %s", name)
}

func SessionURL(port int, name, pane string) (string, error) {
	if err := ValidateSessionName(name); err != nil {
		return "", err
	}
	if pane != "" {
		if err := ValidatePaneID(pane); err != nil {
			return "", err
		}
	}
	u := url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", port), Path: "/sessions/" + name}
	if pane != "" {
		query := url.Values{}
		query.Set("pane", pane)
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}
