/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go-bench",
	Short: "Go benchmarks for Cosmos DB SDK",
	Long:  `Tools to benchmark the performance of the Cosmos DB SDK for Go`,
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
	rootCmd.PersistentFlags().StringP("key", "k", emulatorKey, "Cosmos DB primary key (if not specified, uses Azure CLI credentials)")
	rootCmd.PersistentFlags().StringP("database", "d", "sdk-bench-db", "Benchmarking database name")
}

func getTestDbClient(cmd *cobra.Command, client *azcosmos.Client) (*azcosmos.DatabaseClient, error) {
	databaseName, err := cmd.Flags().GetString("database")
	if err != nil {
		return nil, err
	}
	dbClient, err := client.NewDatabase(databaseName)
	if err != nil {
		return nil, err
	}
	return dbClient, nil
}

func createTestDbClient(cmd *cobra.Command, client *azcosmos.Client) (*azcosmos.DatabaseClient, error) {
	databaseName, err := cmd.Flags().GetString("database")
	if err != nil {
		return nil, err
	}
	resp, err := client.CreateDatabase(cmd.Context(), azcosmos.DatabaseProperties{ID: databaseName}, nil)
	if err != nil {
		return nil, err
	}
	dbClient, err := client.NewDatabase(resp.DatabaseProperties.ID)
	if err != nil {
		return nil, err
	}
	return dbClient, nil
}

func createCosmosClient(cmd *cobra.Command) (*azcosmos.Client, error) {
	endpoint, err := cmd.Flags().GetString("endpoint")
	if err != nil {
		return nil, err
	}
	key, err := cmd.Flags().GetString("key")
	if err != nil {
		return nil, err
	}

	if key != "" {
		cred, err := azcosmos.NewKeyCredential(key)
		if err != nil {
			return nil, err
		}
		return azcosmos.NewClientWithKey(endpoint, cred, nil)
	} else {
		cred, err := azidentity.NewAzureCLICredential(nil)
		if err != nil {
			return nil, err
		}
		return azcosmos.NewClient(endpoint, cred, nil)
	}
}
