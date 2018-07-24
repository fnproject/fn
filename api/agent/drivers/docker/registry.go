package docker

import (
	"net/url"
	"os"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
)

var (
	defaultPrivateRegistries = []string{"hub.docker.com", "index.docker.io"}
)

func registryFromEnv() (map[string]driverAuthConfig, error) {
	drvAuths := make(map[string]driverAuthConfig)

	var auths *docker.AuthConfigurations
	var err error
	if reg := os.Getenv("DOCKER_AUTH"); reg != "" {
		// TODO docker does not use this itself, we should get rid of env docker config (nor is this documented..)
		auths, err = docker.NewAuthConfigurations(strings.NewReader(reg))
	} else {
		auths, err = docker.NewAuthConfigurationsFromDockerCfg()
	}

	if err != nil {
		logrus.WithError(err).Info("no docker auths from config files found (this is fine)")
		return drvAuths, nil
	}

	for key, v := range auths.Configs {

		u, err := url.Parse(v.ServerAddress)
		if err != nil {
			return drvAuths, err
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
