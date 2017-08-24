package flagparser

import (
	"github.com/docker/swarmkit/api"
	"github.com/spf13/pflag"
)

func parseContainer(flags *pflag.FlagSet, spec *api.ServiceSpec) error {
	if flags.Changed("image") {
		image, err := flags.GetString("image")
		if err != nil {
			return err
		}
		spec.Task.GetContainer().Image = image
	}

	if flags.Changed("hostname") {
		hostname, err := flags.GetString("hostname")
		if err != nil {
			return err
		}
		spec.Task.GetContainer().Hostname = hostname
	}

	if flags.Changed("command") {
		command, err := flags.GetStringSlice("command")
		if err != nil {
			return err
		}
		spec.Task.GetContainer().Command = command
	}

	if flags.Changed("args") {
		args, err := flags.GetStringSlice("args")
		if err != nil {
			return err
		}
		spec.Task.GetContainer().Args = args
	}

	if flags.Changed("env") {
		env, err := flags.GetStringSlice("env")
		if err != nil {
			return err
		}
		spec.Task.GetContainer().Env = env
	}

	if flags.Changed("tty") {
		tty, err := flags.GetBool("tty")
		if err != nil {
			return err
		}

		spec.Task.GetContainer().TTY = tty
	}

	if flags.Changed("open-stdin") {
		openStdin, err := flags.GetBool("open-stdin")
		if err != nil {
			return err
		}

		spec.Task.GetContainer().OpenStdin = openStdin
	}

	return nil
}
