package opencode

import "os"

// GetServerCreds reads OpenCode server credentials from environment.
// When no password is set, username is cleared so callers can skip BasicAuth entirely.
func GetServerCreds() (username, password string) {
	password = os.Getenv("OPENCODE_SERVER_PASSWORD")
	if password == "" {
		return "", ""
	}
	username = os.Getenv("OPENCODE_SERVER_USERNAME")
	if username == "" {
		username = "opencode"
	}
	return username, password
}
