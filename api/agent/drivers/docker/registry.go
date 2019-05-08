package docker

import (
	"net/url"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
)

var (
	defaultPrivateRegistries = []string{"hub.docker.com", "index.docker.io"}
)

func registryFromEnv() (map[string]driverAuthConfig, error) {
	// TODO(reed): it's kind of a lot to load the entire docker cli just so we
	// can get this one func, but docker does not seem to provide a nice way to
	// do this from smaller libs/client (go mod cleans up a little)
	cfg, err := config.Load("") // docker initializes from home/env var
	if err != nil {
		logrus.WithError(err).Info("no docker auths from config files found (this is fine)")
		return map[string]driverAuthConfig{}, nil
	}

	return preprocessAuths(cfg.AuthConfigs)
}

func preprocessAuths(auths map[string]types.AuthConfig) (map[string]driverAuthConfig, error) {
	drvAuths := make(map[string]driverAuthConfig)

	for key, v := range auths {

		u, err := url.Parse(v.ServerAddress)
		if err != nil {
			return drvAuths, err
		}

		if u.Scheme == "" {
			// url.Parse won't return an error for urls who do not provide a scheme, and
			// host field will be unset. docker defaults to bare hosts without scheme
			// in its configs, so support this here as well.
			u.Host = v.ServerAddress
		}

		drvAuths[key] = driverAuthConfig{
			auth:       v,
			subdomains: getSubdomains(u.Host),
		}
	}
	return drvAuths, nil
}

func getSubdomains(hostname string) map[string]bool {

	subdomains := make(map[string]bool)
	tokens := strings.Split(hostname, ".")

	if len(tokens) <= 2 {
		subdomains[hostname] = true
	} else {
		for i := 0; i <= len(tokens)-2; i++ {
			joined := strings.Join(tokens[i:], ".")
			subdomains[joined] = true
		}
	}

	return subdomains
}

func findRegistryConfig(reg string, configs map[string]driverAuthConfig) types.AuthConfig {
	var config types.AuthConfig

	if reg != "" {
		res := lookupRegistryConfig(reg, configs)
		if res != nil {
			return *res
		}
	} else {
		for _, reg := range defaultPrivateRegistries {
			res := lookupRegistryConfig(reg, configs)
			if res != nil {
				return *res
			}
		}
	}

	return config
}

func lookupRegistryConfig(reg string, configs map[string]driverAuthConfig) *types.AuthConfig {

	// if any configured host auths match task registry, try them (task docker auth can override)
	for _, v := range configs {
		_, ok := v.subdomains[reg]
		if ok {
			return &v.auth
		}
	}

	return nil
}
