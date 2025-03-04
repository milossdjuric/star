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
			go c.handleStartApp(ctx, name, selectorLabels)
			return nil
		case "stop":
			go c.handleStopApp(ctx, name)
			return nil
		case "query":
			go c.handleQueryApp(ctx, name, selectorLabels)
			return nil
		case "healthcheck":
			go c.handleHealthCheckApp(ctx, name)
			return nil
		case "availabilitycheck":
			go c.handleAvailabilityCheckApp(ctx, name, minReadySeconds)
			return nil
		case "query_healthy":
			go c.handleQueryHealthyApp(ctx, name, selectorLabels)
			return nil
		case "query_available":
			go c.handleQueryAvailableApp(ctx, name, selectorLabels, minReadySeconds)
			return nil
		case "query_all":
			go c.handleQueryAllApp(ctx, name, selectorLabels, minReadySeconds)
			return nil
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

func (c *AppOperationAsyncServer) handleStartApp(ctx context.Context, name string, selectorLabels map[string]string) {

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
		return
	}

	c.client.Publisher.Publish(data, c.nodeId+".app_operation.start_app."+name)
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.start_app."+name)
}

func (c *AppOperationAsyncServer) handleStopApp(ctx context.Context, name string) {
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
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.stop_app."+name)
}

func (c *AppOperationAsyncServer) handleQueryApp(ctx context.Context, prefix string, selectorLabels map[string]string) {
	log.Printf("Querying app containers with prefix: %s and selectorLabels: %v", prefix, selectorLabels)

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

	// when calling docker API, it returns container names with "/" prefix
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
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.query_app."+prefix)

}

func (c *AppOperationAsyncServer) handleHealthCheckApp(ctx context.Context, name string) {
	log.Printf("Health check for container: %s", name)

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
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.healthcheck_app."+name)
}

func (c *AppOperationAsyncServer) handleAvailabilityCheckApp(ctx context.Context, name string, minReadySeconds int64) {
	log.Printf("Availability check for container: %s with minReadySeconds: %d", name, minReadySeconds)

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
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.availabilitycheck_app."+name)
}

func (c *AppOperationAsyncServer) handleQueryHealthyApp(ctx context.Context, prefix string, selectorLabels map[string]string) {
	log.Printf("Querying app containers with prefix: %s and selectorLabels: %v", prefix, selectorLabels)

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
		beforeContainerName, containerName, _ := strings.Cut(container.Names[0], "/")
		if containerName == "" {
			containerName = beforeContainerName
		}

		containerInfo, err := c.dockerClient.ContainerInspect(ctx, containerName)
		if err != nil {
			log.Printf("Failed to inspect container: %v", err)
			errorMessages = append(errorMessages, fmt.Sprintf("Failed to inspect container: %v", err))
		} else {
			if containerInfo.State.Running {
				apps = append(apps, &rusapi.App{Name: containerName, SelectorLabels: container.Labels})
				log.Printf("App found: %s", apps[len(apps)-1].Name)
				log.Printf("Container %s is running", containerName)
			} else {
				log.Printf("Container %s is not running", containerName)
			}
		}
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
	c.client.Publisher.Publish(data, c.nodeId+".app_operation.query_healthy_app."+prefix)
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.query_healthy_app."+prefix)
}

func (c *AppOperationAsyncServer) handleQueryAvailableApp(ctx context.Context, prefix string, selectorLabels map[string]string, minReadySeconds int64) {
	log.Printf("Querying app containers with prefix: %s and selectorLabels: %v", prefix, selectorLabels)

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
	}

	apps := make([]*rusapi.App, 0)
	for _, container := range containers {
		beforeContainerName, containerName, _ := strings.Cut(container.Names[0], "/")
		if containerName == "" {
			containerName = beforeContainerName
		}

		containerInfo, err := c.dockerClient.ContainerInspect(ctx, containerName)
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
						apps = append(apps, &rusapi.App{Name: containerName, SelectorLabels: container.Labels})
					}
				}
			}
		}
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
	c.client.Publisher.Publish(data, c.nodeId+".app_operation.query_available_app."+prefix)
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.query_available_app."+prefix)
}

func (c *AppOperationAsyncServer) handleQueryAllApp(ctx context.Context, prefix string, selectorLabels map[string]string, minReadySeconds int64) {
	log.Printf("Querying app containers with prefix: %s and selectorLabels: %v", prefix, selectorLabels)

	// we use seperate goroutine to handle this operation, since it is only read
	// operation and we dont need to block server thread

	// in this method if error occurs we log it inside goroutine, so we dont block server thread,
	// otherwise we would stil return err just to log it if not nil
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
		// since this is a goroutine, we need to log the error
		log.Printf("Failed to list containers: %v", err)
		errorMessages = append(errorMessages, fmt.Sprintf("Failed to list containers: %v", err))
	}

	totalApps := make([]*rusapi.App, 0)
	readyApps := make([]*rusapi.App, 0)
	availableApps := make([]*rusapi.App, 0)
	for _, container := range containers {
		beforeContainerName, containerName, _ := strings.Cut(container.Names[0], "/")
		if containerName == "" {
			containerName = beforeContainerName
		}

		containerInfo, err := c.dockerClient.ContainerInspect(ctx, containerName)
		if err != nil {
			log.Printf("Failed to inspect container: %v", err)
			errorMessages = append(errorMessages, fmt.Sprintf("Failed to inspect container: %v", err))
		} else {
			totalApps = append(totalApps, &rusapi.App{Name: containerName, SelectorLabels: container.Labels})

			if containerInfo.State.Running {
				readyApps = append(readyApps, &rusapi.App{Name: containerName, SelectorLabels: container.Labels})

				startedAt := containerInfo.State.StartedAt
				startTime, err := time.Parse(time.RFC3339Nano, startedAt)
				if err != nil {
					log.Printf("Failed to parse time: %v", err)
					errorMessages = append(errorMessages, fmt.Sprintf("Failed to parse time: %v", err))
				} else {
					if time.Since(startTime).Seconds() >= float64(minReadySeconds) {
						availableApps = append(availableApps, &rusapi.App{Name: containerName, SelectorLabels: container.Labels})
						log.Printf("Container %s is available", containerName)
					} else {
						// log.Printf("Container %s is not available", containerName)
					}
				}
			}
		}
	}
	response := rusapi.QueryAllAppResp{
		Success:       err == nil,
		ErrorMessages: errorMessages,
		TotalApps:     totalApps,
		ReadyApps:     readyApps,
		AvailableApps: availableApps,
	}

	data, err := proto.Marshal(&response)
	if err != nil {
		// since this is a goroutine, we need to log the error
		log.Printf("Failed to marshal response: %v", err)
		return
	}
	c.client.Publisher.Publish(data, c.nodeId+".app_operation.query_all_app."+prefix)
	log.Println("Response published to NATS topic: ", c.nodeId+".app_operation.query_all_app."+prefix)
}
