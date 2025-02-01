package utils

import (
	"os"
	"strings"
)

const TokenFileName = "token.txt"

func SaveTokenToFile(token string) error {
	return os.WriteFile(TokenFileName, []byte(token), 0600)
}

func ReadTokenFromFile() (string, error) {
	content, err := os.ReadFile(TokenFileName)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}
