package cloudflare

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPagesProjectNameRespectsMaxLength(t *testing.T) {
	long := strings.Repeat("marketing-site", 10)
	name := pagesProjectName(long)
	assert.LessOrEqual(t, len(name), pagesProjectMaxLen)
	assert.True(t, strings.HasPrefix(name, pagesProjectPrefix))
}
