package servers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	rusapi "github.com/milossdjuric/rolling_update_service/pkg/api"
	"google.golang.org/protobuf/proto"
)

type AppOperationAsyncServer struct {
	client       *rusapi.UpdateServiceAsyncClient
	dockerClient *client.Client
	nodeId       string
}

func NewAppOperationAsyncServer(client *rusapi.UpdateServiceAsyncClient, dockerClient *client.Client, nodeId string) (*AppOperationAsyncServer, error) {
	if client == nil {
		return nil, errors.New("client is nil while initializing app config async server")
	}
	return &AppOperationAsyncServer{
		client:       client,
		dockerClient: dockerClient,
		nodeId:       nodeId,
	}, nil
}

func (c *AppOperationAsyncServer) Serve() {
	err := c.client.ReceiveAppOperation(func(orgId, namespace, name, operation string, selectorLabels map[string]string, minReadySeconds int64) error {
		ctx := context.Background()

		log.Println("Received app operation: ", operation)

		switch operation {
		case "start":
			return c.handleStartApp(ctx, name, selectorLabels)
		case "stop":
			return c.handleStopApp(ctx, name)
		case "query":
			return c.handleQueryApp(ctx, name, selectorLabels)
		case "healthcheck":
			return c.handleHealthCheckApp(ctx, name)
		case "availabilitycheck":
			return c.handleAvailabilityCheckApp(ctx, name, minReadySeconds)
		default:
			log.Printf("Unknown operation: %s", operation)
			return fmt.Errorf("unknown operation: %s", operation)
		}
	})

	if err != nil {
		log.Println("Error receiving app operation: ", err)
	}
}

func (c *AppOperationAsyncServer) GracefulStop() {
	c.client.GracefulStop()
}

func (c *AppOperationAsyncServer) handleStartApp(ctx context.Context, name string, selectorLabels map[string]string) error {

	errorMessages := make([]string, 0)

	containerConfig := &container.Config{
		Image:  os.Getenv("DOCKER_CLIENT_IMAGE"),
		Cmd:    []string{"ash", "-c", "while true; do sleep 1000; done"},
		Labels: selectorLabels,
	}

	resp, err := c.dockerClient.ContainerCreate(ctx, containerConfig, nil, nil, nil, name)
	if err != nil {
		errorMessages = append(errorMessages, fmt.Sprintf("Error creating container: %s", err))
		log.Println("Error creating container: ", err)
	} else {
		// log.Println("Container created successfully: ", resp.ID)
	}

	if err := c.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		log.Printf("Error starting container: %s", err)
		errorMessages = append(errorMessages, fmt.Sprintf("Error starting container: %s", err))
	} else {
		// log.Println("Container started successfully: ", resp.ID)
	}

	response := rusapi.StartAppResp{
		Success:       err == nil,
		ErrorMessages: errorMessages,
	}

	data, err := proto.Marshal(&response)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return err
	}

	c.client.Publisher.Publish(data, c.nodeId+".app_operation.start_app."+name)
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.start_app")

	return err
}

func (c *AppOperationAsyncServer) handleStopApp(ctx context.Context, name string) error {

	errorMessages := make([]string, 0)
	err := c.dockerClient.ContainerStop(ctx, name, container.StopOptions{})
	if err != nil {
		log.Printf("Error stopping container: %s", err)
		errorMessages = append(errorMessages, fmt.Sprintf("Error stopping container: %s", err))
	} else {
		// log.Printf("Container %s stopped successfully", name)
	}

	response := rusapi.StopAppResp{
		Success:       err == nil,
		ErrorMessages: errorMessages,
	}

	data, err := proto.Marshal(&response)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
	}

	c.client.Publisher.Publish(data, c.nodeId+".app_operation.stop_app."+name)
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.stop_app")

	return err
}

func (c *AppOperationAsyncServer) handleQueryApp(ctx context.Context, prefix string, selectorLabels map[string]string) error {
	// log.Printf("Querying app containers with prefix: %s and selectorLabels: %v", prefix, selectorLabels)

	errorMessages := make([]string, 0)
	keyValues := make([]filters.KeyValuePair, 0)
	keyValues = append(keyValues, filters.KeyValuePair{Key: "label", Value: "revision=" + prefix})
	for key, value := range selectorLabels {
		keyValue := key + "=" + value
		keyValues = append(keyValues, filters.KeyValuePair{Key: "label", Value: keyValue})
	}
	args := filters.NewArgs(keyValues...)

	containers, err := c.dockerClient.ContainerList(ctx, container.ListOptions{Filters: args})
	if err != nil {
		log.Printf("Failed to list containers: %v", err)
		errorMessages = append(errorMessages, fmt.Sprintf("Failed to list containers: %v", err))
	} else {
		// log.Printf("Found %d containers matching query", len(containers))
	}

	apps := make([]*rusapi.App, 0)
	for _, container := range containers {
		log.Printf("Container found: %v", container)
		beforeContainerName, containerName, _ := strings.Cut(container.Names[0], "/")
		if containerName == "" {
			containerName = beforeContainerName
		}
		apps = append(apps, &rusapi.App{Name: containerName, SelectorLabels: container.Labels})
		log.Printf("App found: %s", apps[len(apps)-1].Name)
	}

	response := rusapi.QueryAppResp{
		Success:       err == nil,
		ErrorMessages: errorMessages,
		Apps:          apps,
	}

	data, err := proto.Marshal(&response)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
	}
	c.client.Publisher.Publish(data, c.nodeId+".app_operation.query_app."+prefix)
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.query_app")

	return err
}

func (c *AppOperationAsyncServer) handleHealthCheckApp(ctx context.Context, name string) error {
	// log.Printf("Health check for container: %s", name)

	errorMessages := make([]string, 0)
	healthy := false
	containerInfo, err := c.dockerClient.ContainerInspect(ctx, name)
	if err != nil {
		log.Printf("Failed to inspect container: %v", err)
		errorMessages = append(errorMessages, fmt.Sprintf("Failed to inspect container: %v", err))
	} else {
		if containerInfo.State.Running {
			healthy = true
			log.Printf("Container %s is running", name)
		} else {
			log.Printf("Container %s is not running", name)
		}
	}

	response := rusapi.HealthCheckAppResp{
		Success:       err == nil,
		ErrorMessages: errorMessages,
		Healthy:       healthy,
	}

	data, err := proto.Marshal(&response)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
	}
	c.client.Publisher.Publish(data, c.nodeId+".app_operation.healthcheck_app."+name)
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.healthcheck_app")

	return err
}

func (c *AppOperationAsyncServer) handleAvailabilityCheckApp(ctx context.Context, name string, minReadySeconds int64) error {
	// log.Printf("Availability check for container: %s with minReadySeconds: %d", name, minReadySeconds)

	errorMessages := make([]string, 0)
	available := false
	containerInfo, err := c.dockerClient.ContainerInspect(ctx, name)
	if err != nil {
		log.Printf("Failed to inspect container: %v", err)
		errorMessages = append(errorMessages, fmt.Sprintf("Failed to inspect container: %v", err))
	} else {
		if containerInfo.State.Running {
			startedAt := containerInfo.State.StartedAt
			startTime, err := time.Parse(time.RFC3339Nano, startedAt)
			if err != nil {
				log.Printf("Failed to parse time: %v", err)
				errorMessages = append(errorMessages, fmt.Sprintf("Failed to parse time: %v", err))
			} else {
				if time.Since(startTime).Seconds() >= float64(minReadySeconds) {
					available = true
					log.Printf("Container %s is available", name)
				} else {
					log.Printf("Container %s is not available", name)
				}
			}
		}
	}

	response := rusapi.AvailabilityCheckAppResp{
		Success:       err == nil,
		ErrorMessages: errorMessages,
		Available:     available,
	}

	data, err := proto.Marshal(&response)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
	}
	c.client.Publisher.Publish(data, c.nodeId+".app_operation.availabilitycheck_app."+name)
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.availabilitycheck_app")

	return err
}
