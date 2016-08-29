package models

type Config map[string]string

func (c *Config) Validate() error {
	return nil
}
