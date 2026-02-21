package cli

import (
	"errors"
	"fmt"
	"log"
	"os"
	"synrax/reporter"

	"github.com/spf13/cobra"
)

var rootCommand = &cobra.Command{
	Use:   "root",
	Short: "", // short description
	Long:  "", // long description
	Run:   func(cmd *cobra.Command, args []string) {},
}

var readDocs = &cobra.Command{
	Use:   "read [repo_id] [file_path]",
	Short: "Executes the unittest given `repo_id` and `file_path`",
	Long:  "Reads a file from the provided path and performs operations on it. Repo id is required to query user's configuration for the app. File path expects API documentation to create a unittest report.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repoID := args[0]
		filePath := args[1]
		log.Printf("cli.read: starting repo_id=%s file_path=%s", repoID, filePath)

		_, err := os.Stat(filePath)
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("Given path does not exist.")
			os.Exit(1)
		}

		if err := reporter.RunUnittest(repoID, filePath); err != nil {
			log.Printf("cli.read: failed repo_id=%s error=%v", repoID, err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		log.Printf("cli.read: completed repo_id=%s", repoID)
	},
}

func init() { // runs automatically at start (go thing)
	rootCommand.AddCommand(readDocs)
}

func Execute() {

	log.Printf("cli.execute: running root command")
	if err := rootCommand.Execute(); err != nil {
		log.Printf("cli.execute: root command failed error=%v", err)
		fmt.Fprintf(os.Stderr, "An error occurred initializing main CLI execution.")
		os.Exit(1)
	}
	log.Printf("cli.execute: root command completed")
}
