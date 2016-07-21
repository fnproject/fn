package server

type Config struct {
	DatabaseURL string `json:"db"`
	Logging     struct {
		To     string `json:"to"`
		Level  string `json:"level"`
		Prefix string `json:"prefix"`
	}
}

func (c *Config) Validate() error {
	// TODO:
	return nil
}
