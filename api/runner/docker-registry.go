package runner

import (
	"strings"
)

type dockerRegistry struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type dockerRegistries []dockerRegistry

func (t dockerRegistries) Find(name string) *dockerRegistry {
	for _, v := range t {
		if strings.HasSuffix(v.Name, name) {
			return &v
		}
	}
	return nil
}
