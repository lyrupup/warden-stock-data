package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

func Sign(secretKey, stringToSign string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func Verify(secretKey, stringToSign, signature string) bool {
	expected := Sign(secretKey, stringToSign)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func BodySHA256Hex(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func BuildStringToSign(method, path, canonicalQuery, secretID, timestamp, nonce string, body []byte) string {
	return strings.Join([]string{
		method,
		path,
		canonicalQuery,
		secretID,
		timestamp,
		nonce,
		BodySHA256Hex(body),
	}, "\n")
}

// CanonicalQuery sorts query keys and joins as a=1&b=2.
func CanonicalQuery(values map[string][]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		for _, v := range values[k] {
			parts = append(parts, k+"="+v)
		}
	}
	return strings.Join(parts, "&")
}

func IsTimestampValid(tsMillis int64, skewSec int) bool {
	now := time.Now().UnixMilli()
	diff := now - tsMillis
	if diff < 0 {
		diff = -diff
	}
	return diff <= int64(skewSec)*1000
}
