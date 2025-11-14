package azurecosmos

/*
#cgo pkg-config: azurecosmos
#include <stdlib.h>
#include "azurecosmos.h"
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

// CosmosError wraps a C.cosmos_error and implements the Go error interface
type CosmosError struct {
	Code    int32
	Message string
}

func (e *CosmosError) Error() string {
	return fmt.Sprintf("Cosmos error %d: %s", e.Code, e.Message)
}

// newCosmosError creates a Go error from a C cosmos_error
func newCosmosError(cerr C.struct_cosmos_error) error {
	if cerr.code == C.COSMOS_ERROR_CODE_SUCCESS {
		return nil
	}

	message := ""
	if cerr.message != nil {
		message = C.GoString(cerr.message)
	}

	return &CosmosError{
		Code:    int32(cerr.code),
		Message: message,
	}
}

// CosmosClient wraps the native cosmos_client pointer
type CosmosClient struct {
	client *C.struct_cosmos_client
}

// DatabaseClient wraps the native cosmos_database_client pointer
type DatabaseClient struct {
	database *C.struct_cosmos_database_client
}

// ContainerClient wraps the native cosmos_container_client pointer
type ContainerClient struct {
	container *C.struct_cosmos_container_client
}

// NewCosmosClientWithKey creates a new CosmosClient using endpoint and key authentication
func NewCosmosClientWithKey(endpoint, key string) (*CosmosClient, error) {
	cEndpoint := C.CString(endpoint)
	defer C.free(unsafe.Pointer(cEndpoint))

	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))

	var client *C.struct_cosmos_client
	var cerr C.struct_cosmos_error

	code := C.cosmos_client_create_with_key(cEndpoint, cKey, &client, &cerr)

	if code != C.COSMOS_ERROR_CODE_SUCCESS {
		return nil, newCosmosError(cerr)
	}

	c := &CosmosClient{client: client}

	// Set finalizer to ensure cleanup
	runtime.SetFinalizer(c, (*CosmosClient).finalize)

	return c, nil
}

// finalize cleans up the native client
func (c *CosmosClient) finalize() {
	if c.client != nil {
		C.cosmos_client_free(c.client)
		c.client = nil
	}
}

// Close explicitly releases the native client resources
func (c *CosmosClient) Close() {
	runtime.SetFinalizer(c, nil)
	c.finalize()
}

// DatabaseClient returns a DatabaseClient for the specified database ID
func (c *CosmosClient) DatabaseClient(databaseID string) (*DatabaseClient, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client is closed")
	}

	cDatabaseID := C.CString(databaseID)
	defer C.free(unsafe.Pointer(cDatabaseID))

	var database *C.struct_cosmos_database_client
	var cerr C.struct_cosmos_error

	code := C.cosmos_client_database_client(c.client, cDatabaseID, &database, &cerr)

	if code != C.COSMOS_ERROR_CODE_SUCCESS {
		return nil, newCosmosError(cerr)
	}

	d := &DatabaseClient{database: database}

	// Set finalizer to ensure cleanup
	runtime.SetFinalizer(d, (*DatabaseClient).finalize)

	return d, nil
}

// finalize cleans up the native database client
func (d *DatabaseClient) finalize() {
	if d.database != nil {
		C.cosmos_database_free(d.database)
		d.database = nil
	}
}

// Close explicitly releases the native database client resources
func (d *DatabaseClient) Close() {
	runtime.SetFinalizer(d, nil)
	d.finalize()
}

// ContainerClient returns a ContainerClient for the specified container ID
func (d *DatabaseClient) ContainerClient(containerID string) (*ContainerClient, error) {
	if d.database == nil {
		return nil, fmt.Errorf("database client is closed")
	}

	cContainerID := C.CString(containerID)
	defer C.free(unsafe.Pointer(cContainerID))

	var container *C.struct_cosmos_container_client
	var cerr C.struct_cosmos_error

	code := C.cosmos_database_container_client(d.database, cContainerID, &container, &cerr)

	if code != C.COSMOS_ERROR_CODE_SUCCESS {
		return nil, newCosmosError(cerr)
	}

	c := &ContainerClient{container: container}

	// Set finalizer to ensure cleanup
	runtime.SetFinalizer(c, (*ContainerClient).finalize)

	return c, nil
}

// finalize cleans up the native container client
func (c *ContainerClient) finalize() {
	if c.container != nil {
		C.cosmos_container_free(c.container)
		c.container = nil
	}
}

// Close explicitly releases the native container client resources
func (c *ContainerClient) Close() {
	runtime.SetFinalizer(c, nil)
	c.finalize()
}

// ReadItem reads an item from the container by ID and partition key, returning the JSON as a string
func (c *ContainerClient) ReadItem(itemID, partitionKey string) (string, error) {
	if c.container == nil {
		return "", fmt.Errorf("container client is closed")
	}

	cItemID := C.CString(itemID)
	defer C.free(unsafe.Pointer(cItemID))

	cPartitionKey := C.CString(partitionKey)
	defer C.free(unsafe.Pointer(cPartitionKey))

	var outJson *C.char
	var cerr C.struct_cosmos_error

	code := C.cosmos_container_read_item(c.container, cPartitionKey, cItemID, &outJson, &cerr)

	if code != C.COSMOS_ERROR_CODE_SUCCESS {
		return "", newCosmosError(cerr)
	}

	if outJson == nil {
		return "", fmt.Errorf("received null JSON response")
	}

	// Convert C string to Go string and free the C memory
	result := C.GoString(outJson)
	C.cosmos_string_free(outJson)

	return result, nil
}
