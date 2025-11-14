package cmd

import (
	"os"

	azurecosmos "github.com/analogrelay/go-rust-interop/go-wrapper"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go-wrapper-bench",
	Short: "Go wrapper benchmarks for Cosmos DB SDK",
	Long:  `Tools to benchmark the performance of the Cosmos DB SDK using the Go wrapper around the Rust native library`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// Well-known Cosmos DB Emulator key, not a secret.
const emulatorKey = "C2y6yDjf5/R+ob0N8A7Cgv30VRDJIWEHLM+4QDU5DE2nQ9nDuVTqobD4b8mGGyPMbIZnqyMsEcaGQy67XIw/Jw=="

func init() {
	rootCmd.PersistentFlags().StringP("endpoint", "e", "https://localhost:8080", "Cosmos DB endpoint URL")
	rootCmd.PersistentFlags().StringP("key", "k", emulatorKey, "Cosmos DB primary key")
	rootCmd.PersistentFlags().StringP("database", "d", "sdk-bench-db", "Benchmarking database name")
}

func createCosmosClient(cmd *cobra.Command) (*azurecosmos.CosmosClient, error) {
	endpoint, err := cmd.Flags().GetString("endpoint")
	if err != nil {
		return nil, err
	}
	key, err := cmd.Flags().GetString("key")
	if err != nil {
		return nil, err
	}

	return azurecosmos.NewCosmosClientWithKey(endpoint, key)
}

func getTestDbClient(cmd *cobra.Command, client *azurecosmos.CosmosClient) (*azurecosmos.DatabaseClient, error) {
	databaseName, err := cmd.Flags().GetString("database")
	if err != nil {
		return nil, err
	}

	return client.DatabaseClient(databaseName)
}
