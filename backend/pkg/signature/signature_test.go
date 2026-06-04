package signature_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/pkg/signature"
)

func buildStringToSign(method, path, query, secretID, ts, nonce string, body []byte) string {
	h := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(h[:])
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n%s",
		method, path, query, secretID, ts, nonce, bodyHash)
}

func TestSignAndVerify(t *testing.T) {
	secretKey := "test-secret-key-48-chars-long-enough!!!!!!"
	method := "GET"
	path := "/open/v1/quotes"
	query := "codes=600000"
	secretID := "AKIDtest123"
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	nonce := "nonce-abc-123"
	body := []byte(nil)

	stringToSign := buildStringToSign(method, path, query, secretID, ts, nonce, body)
	sig := signature.Sign(secretKey, stringToSign)

	require.True(t, signature.Verify(secretKey, stringToSign, sig))
}

func TestVerifyTamperedBody(t *testing.T) {
	secretKey := "test-secret-key"
	stringToSign := buildStringToSign("GET", "/open/v1/quotes", "codes=600000", "AKID1", "1", "n", nil)
	sig := signature.Sign(secretKey, stringToSign)

	tampered := buildStringToSign("GET", "/open/v1/quotes", "codes=600000", "AKID1", "1", "n", []byte("x"))
	require.False(t, signature.Verify(secretKey, tampered, sig))
}

func TestVerifyWrongSecret(t *testing.T) {
	stringToSign := buildStringToSign("GET", "/open/v1/meta", "", "AKID1", "1", "n", nil)
	sig := signature.Sign("key-a", stringToSign)
	require.False(t, signature.Verify("key-b", stringToSign, sig))
}

func TestCanonicalQuery(t *testing.T) {
	require.Equal(t, "", signature.CanonicalQuery(nil))
	require.Equal(t, "a=1&b=2", signature.CanonicalQuery(map[string][]string{
		"b": {"2"},
		"a": {"1"},
	}))
	require.Equal(t, "codes=600000,000001", signature.CanonicalQuery(map[string][]string{
		"codes": {"600000,000001"},
	}))
}

func TestTimestampSkew(t *testing.T) {
	now := time.Now()
	require.True(t, signature.IsTimestampValid(now.UnixMilli(), 300))
	require.False(t, signature.IsTimestampValid(now.Add(-400*time.Second).UnixMilli(), 300))
	require.False(t, signature.IsTimestampValid(now.Add(400*time.Second).UnixMilli(), 300))
}
