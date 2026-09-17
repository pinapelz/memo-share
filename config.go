package main

import "flag"

var listenAddress = flag.String("listen", ":8080", "host:port in which the server will listen")

var expirationOptions = []string{"Never", "1 hour", "4 hours", "1 day", "Custom"}

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
