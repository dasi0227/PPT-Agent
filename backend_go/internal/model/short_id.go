package model

import (
	"crypto/rand"
	"fmt"
)

const shortIDAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"
const shortIDRandomLength = 6

func NewShortID(prefix string) (string, error) {
	if prefix == "" {
		return "", fmt.Errorf("short id prefix is required")
	}
	buf := make([]byte, shortIDRandomLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, shortIDRandomLength)
	for i, b := range buf {
		out[i] = shortIDAlphabet[int(b)%len(shortIDAlphabet)]
	}
	return prefix + "_" + string(out), nil
}

func MustShortID(prefix string) string {
	id, err := NewShortID(prefix)
	if err != nil {
		panic(err)
	}
	return id
}
