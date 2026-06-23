package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSetupSecret = "01234567890123456789012345678901"

func TestLoadDefaults(t *testing.T) {
	// Required values must be present; everything else falls back to defaults.
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("SETUP_SECRET", testSetupSecret)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, 25, cfg.Database.MaxOpenConns)
	assert.False(t, cfg.K8s.InCluster)
	assert.Equal(t, 24*time.Hour, cfg.Auth.TokenExpiry)
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("SETUP_SECRET", testSetupSecret)
	t.Setenv("SERVER_PORT", "9000")
	t.Setenv("SERVER_SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("DATABASE_MAX_OPEN_CONNS", "50")
	t.Setenv("K8S_IN_CLUSTER", "true")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 9000, cfg.Server.Port)
	assert.Equal(t, 30*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.True(t, cfg.K8s.InCluster)
}

func TestLoadInvalidNumbersFallBack(t *testing.T) {
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("SETUP_SECRET", testSetupSecret)
	t.Setenv("SERVER_PORT", "not-a-number")
	t.Setenv("K8S_IN_CLUSTER", "not-a-bool")
	t.Setenv("SERVER_SHUTDOWN_TIMEOUT", "not-a-duration")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.False(t, cfg.K8s.InCluster)
	assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout)
}

func TestLoadMissingJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("SETUP_SECRET", testSetupSecret)
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoadJWTSecretTooShort(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("SETUP_SECRET", testSetupSecret)
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32 characters")
}

func TestLoadMissingSetupSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("SETUP_SECRET", "")
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SETUP_SECRET")
}

func TestLoadSetupSecretTooShort(t *testing.T) {
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("SETUP_SECRET", "short")
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SETUP_SECRET")
}

func TestListenAddr(t *testing.T) {
	c := &Config{Server: ServerConfig{Host: "127.0.0.1", Port: 8080}}
	assert.Equal(t, "127.0.0.1:8080", c.ListenAddr())
}
