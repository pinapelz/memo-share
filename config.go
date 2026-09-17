package main

import (
	"bufio"
	"flag"
	"os"
	"strings"
)

var listenAddress = flag.String("listen", ":8080", "host:port in which the server will listen")

var expirationOptions = []string{"Never", "1 hour", "4 hours", "1 day", "Custom"}

type AuthConfig struct {
	Enabled  bool
	Password string
	Realm    string
}

var authConfig AuthConfig

func loadAuthConfigFromEnv() AuthConfig {
	password := os.Getenv("AUTH_PASSWORD")
	if password == "" {
		return AuthConfig{Enabled: false}
	}

	return AuthConfig{
		Enabled:  true,
		Password: password,
	}
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, "\"")
		value = strings.Trim(value, "'")
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	_ = scanner.Err()
}

func applyDefaultExpiry(customExpiry string) {
	switch customExpiry {
	case "1d":
		expirationOptions = []string{"1 day", "Never", "1 hour", "4 hours", "Custom"}
	case "4h":
		expirationOptions = []string{"4 hours", "Never", "1 hour", "1 day", "Custom"}
	case "1h":
		expirationOptions = []string{"1 hour", "Never", "4 hours", "1 day", "Custom"}
	default:
		expirationOptions = append([]string{customExpiry}, expirationOptions...)
	}
}
