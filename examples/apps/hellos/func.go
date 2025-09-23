package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
)

type link struct {
	Text string
	Href string
}

func main() {
	const tpl = `
	<!DOCTYPE html>
	<html>
		<head>
			<meta charset="UTF-8">
			<title>{{.Title}}</title>
		</head>
		<body>
			<h1>{{.Title}}</h1>
			<p>{{.Body}}</p>
			<div>
				{{range .Items}}<div><a href="{{.Href}}">{{ .Text }}</a></div>{{else}}<div><strong>no rows</strong></div>{{end}}
			</div>
		</body>
	</html>`

	check := func(err error) {
		if err != nil {
			log.Fatal(err)
		}
	}
	t, err := template.New("webpage").Parse(tpl)
	check(err)

	appName := os.Getenv("FN_APP_NAME")

	data := struct {
		Title string
		Body  string
		Items []link
	}{
		Title: "My App",
		Body:  "This is my app. It may not be the best app, but it's my app. And it's multilingual!",
		Items: []link{
			{"Ruby", fmt.Sprintf("/r/%s/ruby", appName)},
			{"Node", fmt.Sprintf("/r/%s/node", appName)},
			{"Python", fmt.Sprintf("/r/%s/python", appName)},
		},
	}

	err = t.Execute(os.Stdout, data)
	check(err)
}
