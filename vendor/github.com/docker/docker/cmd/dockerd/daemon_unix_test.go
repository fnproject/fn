// +build !windows

package main

import (
	"testing"

	"github.com/docker/docker/daemon/config"
	"github.com/gotestyourself/gotestyourself/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDaemonCliConfigWithDaemonFlags(t *testing.T) {
	content := `{"log-opts": {"max-size": "1k"}}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	opts.Debug = true
	opts.LogLevel = "info"
	assert.NoError(t, opts.flags.Set("selinux-enabled", "true"))

	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)

	assert.True(t, loadedConfig.Debug)
	assert.Equal(t, "info", loadedConfig.LogLevel)
	assert.True(t, loadedConfig.EnableSelinuxSupport)
	assert.Equal(t, "json-file", loadedConfig.LogConfig.Type)
	assert.Equal(t, "1k", loadedConfig.LogConfig.Config["max-size"])
}

func TestLoadDaemonConfigWithNetwork(t *testing.T) {
	content := `{"bip": "127.0.0.2", "ip": "127.0.0.1"}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)

	assert.Equal(t, "127.0.0.2", loadedConfig.IP)
	assert.Equal(t, "127.0.0.1", loadedConfig.DefaultIP.String())
}

func TestLoadDaemonConfigWithMapOptions(t *testing.T) {
	content := `{
		"cluster-store-opts": {"kv.cacertfile": "/var/lib/docker/discovery_certs/ca.pem"},
		"log-opts": {"tag": "test"}
}`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)
	assert.NotNil(t, loadedConfig.ClusterOpts)

	expectedPath := "/var/lib/docker/discovery_certs/ca.pem"
	assert.Equal(t, expectedPath, loadedConfig.ClusterOpts["kv.cacertfile"])
	assert.NotNil(t, loadedConfig.LogConfig.Config)
	assert.Equal(t, "test", loadedConfig.LogConfig.Config["tag"])
}

func TestLoadDaemonConfigWithTrueDefaultValues(t *testing.T) {
	content := `{ "userland-proxy": false }`
	tempFile := fs.NewFile(t, "config", fs.WithContent(content))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)

	assert.False(t, loadedConfig.EnableUserlandProxy)

	// make sure reloading doesn't generate configuration
	// conflicts after normalizing boolean values.
	reload := func(reloadedConfig *config.Config) {
		assert.False(t, reloadedConfig.EnableUserlandProxy)
	}
	assert.NoError(t, config.Reload(opts.configFile, opts.flags, reload))
}

func TestLoadDaemonConfigWithTrueDefaultValuesLeaveDefaults(t *testing.T) {
	tempFile := fs.NewFile(t, "config", fs.WithContent(`{}`))
	defer tempFile.Remove()

	opts := defaultOptions(tempFile.Path())
	loadedConfig, err := loadDaemonCliConfig(opts)
	require.NoError(t, err)
	require.NotNil(t, loadedConfig)

	assert.True(t, loadedConfig.EnableUserlandProxy)
}
