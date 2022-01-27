package main

import "encoding/base64"

func base64decode(s string) string {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err.Error()
	}
	return string(decoded)
}
