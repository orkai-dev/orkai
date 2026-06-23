package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionConstants(t *testing.T) {
	assert.NotEmpty(t, Version)
	assert.Equal(t, "Orkai", Name)
	assert.Equal(t, "orkai-dev", GitHubOwner)
	assert.Equal(t, "orkai", GitHubRepo)
	assert.Contains(t, Website, "github.com")
}
