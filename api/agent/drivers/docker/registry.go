package docker

import (
	"fmt"
	"net/url"
	"os"
	"os/user"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
)

var (
	defaultPrivateRegistries = []string{"hub.docker.com", "index.docker.io"}
)

func registryFromEnv() (map[string]driverAuthConfig, error) {
	var auths *docker.AuthConfigurations
	var err error
	if reg := os.Getenv("FN_DOCKER_AUTH"); reg != "" {
		auths, err = docker.NewAuthConfigurations(strings.NewReader(reg))
	} else {
		auths, err = newAuthConfigurationsFromPodmanCfg()
		if err != nil {
			logrus.WithError(err).Info("Failed to parse podman config. Fall back to docker config.")
			auths, err = docker.NewAuthConfigurationsFromDockerCfg()
		}
	}

	if err != nil {
		logrus.WithError(err).Info("no docker or podman auths from config files found (this is fine)")
		return map[string]driverAuthConfig{}, nil
	}

	return preprocessAuths(auths)
}

func newAuthConfigurationsFromPodmanCfg() (*docker.AuthConfigurations, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}

	authJson := fmt.Sprintf("/run/user/%s/containers/auth.json", currentUser)
	_, err = os.Stat(authJson)
	if err != nil && os.IsNotExist(err) {
		return nil, err
	}
	auths, err := docker.NewAuthConfigurations(strings.NewReader(authJson))
	return auths, err
}

func preprocessAuths(auths *docker.AuthConfigurations) (map[string]driverAuthConfig, error) {
	drvAuths := make(map[string]driverAuthConfig)

	for key, v := range auths.Configs {

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

func findRegistryConfig(reg string, configs map[string]driverAuthConfig) *docker.AuthConfiguration {
	var config docker.AuthConfiguration

	if reg != "" {
		res := lookupRegistryConfig(reg, configs)
		if res != nil {
			return res
		}
	} else {
		for _, reg := range defaultPrivateRegistries {
			res := lookupRegistryConfig(reg, configs)
			if res != nil {
				return res
			}
		}
	}

	return &config
}

func lookupRegistryConfig(reg string, configs map[string]driverAuthConfig) *docker.AuthConfiguration {

	// if any configured host auths match task registry, try them (task docker auth can override)
	for _, v := range configs {
		_, ok := v.subdomains[reg]
		if ok {
			return &v.auth
		}
	}

	return nil
}
