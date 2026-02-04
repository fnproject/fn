package test

import (
	"fmt"
	"os"
)

const EnvRemoteDockerRepo = "REMOTE_DOCKER_REPO"
const EnvTestUtilsImage = "TEST_UTILS_IMAGE"
const EnvFnStatusCheckerImage = "FN_STATUS_CHECKER_IMAGE"

func GetTestUtilsImage() string {
	if testUtilsImage := os.Getenv(EnvTestUtilsImage); testUtilsImage != "" {
		return testUtilsImage
	}
	return "fnproject/fn-test-utils"
}

func GetStatusCheckerImage() string {
	if image := os.Getenv(EnvFnStatusCheckerImage); image != "" {
		return image
	}
	return "fnproject/fn-status-checker:latest"
}

func GetPublicImage(imageName string) string {
	if remoteDockerRepo := os.Getenv(EnvRemoteDockerRepo); remoteDockerRepo != "" {
		return fmt.Sprintf("%s/%s", remoteDockerRepo, imageName)
	}
	return imageName
}

func GetBusyBoxImage() string {
	return GetPublicImage("busybox")
}
