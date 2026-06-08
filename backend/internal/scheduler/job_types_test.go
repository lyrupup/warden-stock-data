package scheduler

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatFailedCodesEmpty(t *testing.T) {
	require.Equal(t, "", formatFailedCodes(nil))
	require.Equal(t, "", formatFailedCodes([]string{}))
}

func TestFormatFailedCodesJoin(t *testing.T) {
	require.Equal(t, "600000,000001", formatFailedCodes([]string{"600000", "000001"}))
}

func TestFormatFailedCodesTruncate(t *testing.T) {
	codes := make([]string, maxFailedCodesStored+10)
	for i := range codes {
		codes[i] = "c"
	}
	out := formatFailedCodes(codes)
	require.Contains(t, out, "…(共 510 个)")
	// 截断后实际保留的代码数应为上限值（每个代码为单字符 "c"，标注中不含 "c"）。
	require.Equal(t, maxFailedCodesStored, strings.Count(out, "c"))
}
