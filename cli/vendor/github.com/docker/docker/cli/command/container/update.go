package container

import (
	"fmt"
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type updateOptions struct {
	blkioWeight        uint16
	cpuPeriod          int64
	cpuQuota           int64
	cpuRealtimePeriod  int64
	cpuRealtimeRuntime int64
	cpusetCpus         string
	cpusetMems         string
	cpuShares          int64
	memory             opts.MemBytes
	memoryReservation  opts.MemBytes
	memorySwap         opts.MemSwapBytes
	kernelMemory       opts.MemBytes
	restartPolicy      string
	cpus               opts.NanoCPUs

	nFlag int

	containers []string
}

// NewUpdateCommand creates a new cobra.Command for `docker update`
func NewUpdateCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts updateOptions

	cmd := &cobra.Command{
		Use:   "update [OPTIONS] CONTAINER [CONTAINER...]",
		Short: "Update configuration of one or more containers",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			opts.nFlag = cmd.Flags().NFlag()
			return runUpdate(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.Uint16Var(&opts.blkioWeight, "blkio-weight", 0, "Block IO (relative weight), between 10 and 1000, or 0 to disable (default 0)")
	flags.Int64Var(&opts.cpuPeriod, "cpu-period", 0, "Limit CPU CFS (Completely Fair Scheduler) period")
	flags.Int64Var(&opts.cpuQuota, "cpu-quota", 0, "Limit CPU CFS (Completely Fair Scheduler) quota")
	flags.Int64Var(&opts.cpuRealtimePeriod, "cpu-rt-period", 0, "Limit the CPU real-time period in microseconds")
	flags.SetAnnotation("cpu-rt-period", "version", []string{"1.25"})
	flags.Int64Var(&opts.cpuRealtimeRuntime, "cpu-rt-runtime", 0, "Limit the CPU real-time runtime in microseconds")
	flags.SetAnnotation("cpu-rt-runtime", "version", []string{"1.25"})
	flags.StringVar(&opts.cpusetCpus, "cpuset-cpus", "", "CPUs in which to allow execution (0-3, 0,1)")
	flags.StringVar(&opts.cpusetMems, "cpuset-mems", "", "MEMs in which to allow execution (0-3, 0,1)")
	flags.Int64VarP(&opts.cpuShares, "cpu-shares", "c", 0, "CPU shares (relative weight)")
	flags.VarP(&opts.memory, "memory", "m", "Memory limit")
	flags.Var(&opts.memoryReservation, "memory-reservation", "Memory soft limit")
	flags.Var(&opts.memorySwap, "memory-swap", "Swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	flags.Var(&opts.kernelMemory, "kernel-memory", "Kernel memory limit")
	flags.StringVar(&opts.restartPolicy, "restart", "", "Restart policy to apply when a container exits")

	flags.Var(&opts.cpus, "cpus", "Number of CPUs")
	flags.SetAnnotation("cpus", "version", []string{"1.29"})

	return cmd
}

func runUpdate(dockerCli *command.DockerCli, opts *updateOptions) error {
	var err error

	if opts.nFlag == 0 {
		return errors.New("You must provide one or more flags when using this command.")
	}

	var restartPolicy containertypes.RestartPolicy
	if opts.restartPolicy != "" {
		restartPolicy, err = runconfigopts.ParseRestartPolicy(opts.restartPolicy)
		if err != nil {
			return err
		}
	}

	resources := containertypes.Resources{
		BlkioWeight:        opts.blkioWeight,
		CpusetCpus:         opts.cpusetCpus,
		CpusetMems:         opts.cpusetMems,
		CPUShares:          opts.cpuShares,
		Memory:             opts.memory.Value(),
		MemoryReservation:  opts.memoryReservation.Value(),
		MemorySwap:         opts.memorySwap.Value(),
		KernelMemory:       opts.kernelMemory.Value(),
		CPUPeriod:          opts.cpuPeriod,
		CPUQuota:           opts.cpuQuota,
		CPURealtimePeriod:  opts.cpuRealtimePeriod,
		CPURealtimeRuntime: opts.cpuRealtimeRuntime,
		NanoCPUs:           opts.cpus.Value(),
	}

	updateConfig := containertypes.UpdateConfig{
		Resources:     resources,
		RestartPolicy: restartPolicy,
	}

	ctx := context.Background()

	var (
		warns []string
		errs  []string
	)
	for _, container := range opts.containers {
		r, err := dockerCli.Client().ContainerUpdate(ctx, container, updateConfig)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintln(dockerCli.Out(), container)
		}
		warns = append(warns, r.Warnings...)
	}
	if len(warns) > 0 {
		fmt.Fprintln(dockerCli.Out(), strings.Join(warns, "\n"))
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}
