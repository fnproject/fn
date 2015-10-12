package main

type Route struct {
	// TODO: Change destinations to a simple cache so it can expire entries after 55 minutes (the one we use in common?)
	Host         string   `json:"host"`
	Destinations []string `json:"destinations"`
	ProjectId    string   `json:"project_id"`
	Token        string   `json:"token"` // store this so we can queue up new workers on demand
	CodeName     string   `json:"code_name"`
}

// for adding new hosts
type Route2 struct {
	Host string `json:"host"`
	Dest string `json:"dest"`
}

// An app is that base object for api gateway
type App struct {
	Name      string   `json:"name"`
	ProjectId string   `json:"project_id"`
	Routes    []Route3 `json:"routes"`
}

// this is for the new api gateway
type Route3 struct {
	Path          string `json:"path"` // run/app
	Image         string `json:"image"`
	Type          string `json:"type"`
	ContainerPath string `json:"cpath"`
}
