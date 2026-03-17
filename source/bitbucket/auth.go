package bitbucket

import (
	"encoding/base64"
	"fmt"
)

// Authenticate returns an auth header string using Basic Auth (API Token).
func Authenticate(email, tokenStr string) (string, error) {
	if email != "" && tokenStr != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(email + ":" + tokenStr))
		return "Basic " + encoded, nil
	}

	return "", fmt.Errorf("no authentication method available; set BITBUCKET_EMAIL + BITBUCKET_TOKEN")
}
