package service

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
)

func newCreateCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := newServiceOptions()

	cmd := &cobra.Command{
		Use:   "create [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short: "Create a new service",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.image = args[0]
			if len(args) > 1 {
				opts.args = args[1:]
			}
			return runCreate(dockerCli, cmd.Flags(), opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.mode, flagMode, "replicated", "Service mode (replicated or global)")
	flags.StringVar(&opts.name, flagName, "", "Service name")

	addServiceFlags(flags, opts, buildServiceDefaultFlagMapping())

	flags.VarP(&opts.labels, flagLabel, "l", "Service labels")
	flags.Var(&opts.containerLabels, flagContainerLabel, "Container labels")
	flags.VarP(&opts.env, flagEnv, "e", "Set environment variables")
	flags.Var(&opts.envFile, flagEnvFile, "Read in a file of environment variables")
	flags.Var(&opts.mounts, flagMount, "Attach a filesystem mount to the service")
	flags.Var(&opts.constraints, flagConstraint, "Placement constraints")
	flags.Var(&opts.placementPrefs, flagPlacementPref, "Add a placement preference")
	flags.SetAnnotation(flagPlacementPref, "version", []string{"1.28"})
	flags.Var(&opts.networks, flagNetwork, "Network attachments")
	flags.Var(&opts.secrets, flagSecret, "Specify secrets to expose to the service")
	flags.SetAnnotation(flagSecret, "version", []string{"1.25"})
	flags.VarP(&opts.endpoint.publishPorts, flagPublish, "p", "Publish a port as a node port")
	flags.Var(&opts.groups, flagGroup, "Set one or more supplementary user groups for the container")
	flags.SetAnnotation(flagGroup, "version", []string{"1.25"})
	flags.Var(&opts.dns, flagDNS, "Set custom DNS servers")
	flags.SetAnnotation(flagDNS, "version", []string{"1.25"})
	flags.Var(&opts.dnsOption, flagDNSOption, "Set DNS options")
	flags.SetAnnotation(flagDNSOption, "version", []string{"1.25"})
	flags.Var(&opts.dnsSearch, flagDNSSearch, "Set custom DNS search domains")
	flags.SetAnnotation(flagDNSSearch, "version", []string{"1.25"})
	flags.Var(&opts.hosts, flagHost, "Set one or more custom host-to-IP mappings (host:ip)")
	flags.SetAnnotation(flagHost, "version", []string{"1.25"})

	flags.SetInterspersed(false)
	return cmd
}

func runCreate(dockerCli *command.DockerCli, flags *pflag.FlagSet, opts *serviceOptions) error {
	apiClient := dockerCli.Client()
	createOpts := types.ServiceCreateOptions{}

	ctx := context.Background()

	service, err := opts.ToService(ctx, apiClient, flags)
	if err != nil {
		return err
	}

	specifiedSecrets := opts.secrets.Value()
	if len(specifiedSecrets) > 0 {
		// parse and validate secrets
		secrets, err := ParseSecrets(apiClient, specifiedSecrets)
		if err != nil {
			return err
		}
		service.TaskTemplate.ContainerSpec.Secrets = secrets

	}

	if err := resolveServiceImageDigest(dockerCli, &service); err != nil {
		return err
	}

	// only send auth if flag was set
	if opts.registryAuth {
		// Retrieve encoded auth token from the image reference
		encodedAuth, err := command.RetrieveAuthTokenFromImage(ctx, dockerCli, opts.image)
		if err != nil {
			return err
		}
		createOpts.EncodedRegistryAuth = encodedAuth
	}

	response, err := apiClient.ServiceCreate(ctx, service, createOpts)
	if err != nil {
		return err
	}

	for _, warning := range response.Warnings {
		fmt.Fprintln(dockerCli.Err(), warning)
	}

	fmt.Fprintf(dockerCli.Out(), "%s\n", response.ID)

	if opts.detach {
		if !flags.Changed("detach") {
			fmt.Fprintln(dockerCli.Err(), "Since --detach=false was not specified, tasks will be created in the background.\n"+
				"In a future release, --detach=false will become the default.")
		}
		return nil
	}

	return waitOnService(ctx, dockerCli, response.ID, opts)
}
