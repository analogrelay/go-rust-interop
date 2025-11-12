/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/spf13/cobra"
)

// createDbCmd represents the createDb command
var createDbCmd = &cobra.Command{
	Use:   "createDb",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := createCosmosClient(cmd)
		if err != nil {
			fmt.Println("Error creating Cosmos client:", err)
			return
		}
		dbClient, err := createTestDbClient(cmd, client)
		if err != nil {
			fmt.Println("Error creating database:", err)
			return
		}

		containerProperties := azcosmos.ContainerProperties{
			ID: "RandomDocs",
			PartitionKeyDefinition: azcosmos.PartitionKeyDefinition{
				Paths: []string{"/partitionKey"},
				Kind:  azcosmos.PartitionKeyKindHash,
			},
		}
		throughputProperties := azcosmos.NewManualThroughputProperties(40000)
		_, err = dbClient.CreateContainer(cmd.Context(), containerProperties, &azcosmos.CreateContainerOptions{ThroughputProperties: &throughputProperties})
		if err != nil {
			fmt.Println("Error creating container:", err)
			return
		}
		containerClient, err := dbClient.NewContainer("RandomDocs")
		if err != nil {
			fmt.Println("Error getting container client:", err)
			return
		}
		fmt.Println("Database and container created successfully.")

		const itemCount = 10000
		const partitionCount = 10

		fmt.Printf("Inserting %d sample documents across %d partitions...\n", itemCount, partitionCount)
		err = insertSampleDocuments(cmd, containerClient, itemCount, partitionCount)
		if err != nil {
			fmt.Println("Error inserting sample documents:", err)
			return
		}
	},
}

func insertSampleDocuments(cmd *cobra.Command, dbClient *azcosmos.ContainerClient, itemCount, partitionCount int) error {
	// Insert concurrently using goroutines
	concurrency := 32
	jobs := make(chan int, itemCount)
	results := make(chan error, itemCount)

	var wg sync.WaitGroup
	fmt.Printf("Starting insertion with %d concurrent workers...\n", concurrency)
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			fmt.Printf("Worker %d started\n", w)
			defer wg.Done()
			for j := range jobs {
				if j%1000 == 0 {
					fmt.Printf("Worker %d inserting item %d...\n", w, j)
				}
				item := createRandomDocsItem(j, partitionCount)
				itemBytes, err := json.Marshal(item)
				if err != nil {
					results <- err
					continue
				}
				pk := azcosmos.NewPartitionKeyString(item.PartitionKey)
				_, err = dbClient.CreateItem(cmd.Context(), pk, itemBytes, nil)
				results <- err
			}
			fmt.Printf("Worker %d complete\n", w)
		}()
	}

	for i := 0; i < itemCount; i++ {
		jobs <- i
	}
	close(jobs)

	fmt.Println("Waiting for all insertions to complete...")

	wg.Wait()

	fmt.Println("Insertions complete, collecting results...")

	errs := []error{}
	close(results)
	for res := range results {
		if res != nil {
			errs = append(errs, res)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

type RandomDocsItem struct {
	ID           string `json:"id"`
	PartitionKey string `json:"partitionKey"`
	Data         string `json:"data"`
	RandomNumber int    `json:"randomNumber"`
}

func createRandomDocsItem(index, partitionCount int) RandomDocsItem {
	return RandomDocsItem{
		ID:           fmt.Sprintf("item%d", index),
		PartitionKey: fmt.Sprintf("partition%d", index%partitionCount),
		Data:         generateRandomString(1024), // 1KB of random data
		RandomNumber: generateRandomInt(0, 10000),
	}
}

func generateRandomString(size int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, size)
	for i := range b {
		b[i] = letters[generateRandomInt(0, len(letters)-1)]
	}
	return string(b)
}

func generateRandomInt(min, max int) int {
	return min + rand.Intn(max-min+1)
}

func init() {
	rootCmd.AddCommand(createDbCmd)
}
