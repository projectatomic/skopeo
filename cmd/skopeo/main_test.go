package main

import (
	"bytes"
)

// runSkopeo creates an app object and runs it with args, with an implied first "skopeo".
// Returns output intended for stdout and the returned error, if any.
func runSkopeo(args ...string) (string, error) {
	app, _ := createApp()
	stdout := bytes.Buffer{}
	app.SetOut(&stdout)
	app.SetArgs(args)
	err := app.Execute()
	return stdout.String(), err
}
