package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

func SplitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func NewSecretID() (string, error) {
	h, err := RandomHex(16)
	if err != nil {
		return "", err
	}
	return "AKID" + h, nil
}

func NewSecretKey() (string, error) {
	h, err := RandomHex(24)
	if err != nil {
		return "", err
	}
	return h, nil
}

func Ptr[T any](v T) *T { return &v }

func DefaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func FormatDecimalMap(m map[string]interface{}) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = fmt.Sprint(v)
	}
	return out
}
