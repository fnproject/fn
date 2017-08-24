package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/docker/swarmkit/cmd/swarmd/defaults"
	"github.com/spf13/cobra"
)

var (
	mainCmd = &cobra.Command{
		Use:   os.Args[0],
		Short: "Tool to translate and decrypt the raft logs of a swarm manager",
	}

	decryptCmd = &cobra.Command{
		Use:   "decrypt <output directory>",
		Short: "Decrypt a swarm manager's raft logs to an optional directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("%s command does not take any arguments", os.Args[0])
			}

			outDir, err := cmd.Flags().GetString("output-dir")
			if err != nil {
				return err
			}

			stateDir, err := cmd.Flags().GetString("state-dir")
			if err != nil {
				return err
			}

			unlockKey, err := cmd.Flags().GetString("unlock-key")
			if err != nil {
				return err
			}

			return decryptRaftData(stateDir, outDir, unlockKey)
		},
	}

	dumpWALCmd = &cobra.Command{
		Use:   "dump-wal",
		Short: "Display entries from the Raft log",
		RunE: func(cmd *cobra.Command, args []string) error {
			stateDir, err := cmd.Flags().GetString("state-dir")
			if err != nil {
				return err
			}

			unlockKey, err := cmd.Flags().GetString("unlock-key")
			if err != nil {
				return err
			}

			start, err := cmd.Flags().GetUint64("start")
			if err != nil {
				return err
			}

			end, err := cmd.Flags().GetUint64("end")
			if err != nil {
				return err
			}

			return dumpWAL(stateDir, unlockKey, start, end)
		},
	}

	dumpSnapshotCmd = &cobra.Command{
		Use:   "dump-snapshot",
		Short: "Display entries from the latest Raft snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			stateDir, err := cmd.Flags().GetString("state-dir")
			if err != nil {
				return err
			}

			unlockKey, err := cmd.Flags().GetString("unlock-key")
			if err != nil {
				return err
			}

			return dumpSnapshot(stateDir, unlockKey)
		},
	}

	dumpObjectCmd = &cobra.Command{
		Use:   "dump-object [type]",
		Short: "Display an object from the Raft snapshot/WAL",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("dump-object subcommand takes exactly 1 argument")
			}

			stateDir, err := cmd.Flags().GetString("state-dir")
			if err != nil {
				return err
			}

			unlockKey, err := cmd.Flags().GetString("unlock-key")
			if err != nil {
				return err
			}

			selector := objSelector{all: true}

			id, err := cmd.Flags().GetString("id")
			if err != nil {
				return err
			}
			if id != "" {
				selector.id = id
				selector.all = false
			}

			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name != "" {
				selector.name = name
				selector.all = false
			}

			return dumpObject(stateDir, unlockKey, args[0], selector)
		},
	}
)

func init() {
	mainCmd.PersistentFlags().StringP("state-dir", "d", defaults.StateDir, "State directory")
	mainCmd.PersistentFlags().String("unlock-key", "", "Unlock key, if raft logs are encrypted")
	decryptCmd.Flags().StringP("output-dir", "o", "plaintext_raft", "Output directory for decrypted raft logs")
	mainCmd.AddCommand(
		decryptCmd,
		dumpWALCmd,
		dumpSnapshotCmd,
		dumpObjectCmd,
	)

	dumpWALCmd.Flags().Uint64("start", 0, "Start of index range to dump")
	dumpWALCmd.Flags().Uint64("end", 0, "End of index range to dump")

	dumpObjectCmd.Flags().String("id", "", "Look up object by ID")
	dumpObjectCmd.Flags().String("name", "", "Look up object by name")
}

func main() {
	if _, err := mainCmd.ExecuteC(); err != nil {
		os.Exit(-1)
	}
}
