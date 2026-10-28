package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTokenConcurrentLimitDefaultExcludedName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{name: "rerank", want: true},
		{name: "reranker-public", want: true},
		{name: "Embed-API", want: true},
		{name: "text-embedding", want: true},
		{name: "shared-LOBEHUB-key", want: true},
		{name: "lobehub", want: true},
		{name: "claude-sonnet", want: false},
		{name: "gemini-pro", want: false},
		{name: "", want: false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, IsTokenConcurrentLimitDefaultExcludedName(tc.name), tc.name)
	}
}
