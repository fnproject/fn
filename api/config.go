package api

type Config struct {
	CloudFlare struct {
		Email  string `json:"email"`
		ApiKey string `json:"api_key"`
		ZoneId string `json:"zone_id"`
	} `json:"cloudflare"`
	Cache struct {
		Host      string `json:"host"`
		Token     string `json:"token"`
		ProjectId string `json:"project_id"`
	}
	Iron struct {
		Token      string `json:"token"`
		ProjectId  string `json:"project_id"`
		SuperToken string `json:"super_token"`
		WorkerHost string `json:"worker_host"`
		AuthHost   string `json:"auth_host"`
	} `json:"iron"`
	Logging struct {
		To     string `json:"to"`
		Level  string `json:"level"`
		Prefix string `json:"prefix"`
	}
}

func (c *Config) Validate() error {
	// TODO:
	return nil
}
