package cli

import (
	"errors"
	"fmt"
	"log"
	"os"
	"synrax/reporter"
	"synrax/toolkit"

	"github.com/spf13/cobra"
)

var rootCommand = &cobra.Command{
	Use:   "root",
	Short: "", // short description #!#
	Long:  "", // long description #!#
	Run:   func(cmd *cobra.Command, args []string) {},
}

var readDocs = &cobra.Command{
	Use:   "read [repo_id] [file_path]",
	Short: "Executes the unittest given `repo_id` and `file_path`",
	Long:  "Reads a file from the provided path and performs operations on it. Repo id is required to query user's configuration for the app. File path expects API documentation to create a unittest report.",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		repoID := args[0]
		filePath := args[1]
		oidcToken := args[2]
		log.Printf("cli.read: starting repo_id=%s file_path=%s", repoID, filePath)

		// ---- parameter validation ---

		// 1) File Path validation
		_, err := os.Stat(filePath)
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("Given path does not exist.")
			os.Exit(1)
		}

		// 2) Validate Token by Calling server
		log.Printf("runner: config fetched repo_id=%s", repoID)
		valid, err := toolkit.SynraxOIDCCaller(repoID, oidcToken)
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
		if !valid {
			log.Println("Passed invalid OIDC token.")
			os.Exit(1)
		}

		// 3) get config from DB
		log.Printf("runner: start repo_id=%s file=%s", repoID, filePath)
		config, err := toolkit.SynraxConfigCaller(repoID)
		if err != nil {
			log.Printf("runner: config fetch failed repo_id=%s error=%v", repoID, err)
			os.Exit(1)
		}
		// 4) Checks passed. Run main function to gather unittest report
		if err := reporter.RunUnittest(filePath, config); err != nil {
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
