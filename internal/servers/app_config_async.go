package servers

import (
	"errors"
	"fmt"
	"log"
	"time"

	meridianapi "github.com/c12s/meridian/pkg/api"
	"github.com/c12s/star/internal/services"
)

type AppConfigAsyncServer struct {
	client *meridianapi.MeridianAsyncClient
	serf   *services.SerfAgent
	nodeId string
}

func NewAppConfigAsyncServer(client *meridianapi.MeridianAsyncClient, serf *services.SerfAgent, nodeId string) (*AppConfigAsyncServer, error) {
	if client == nil {
		return nil, errors.New("client is nil while initializing app config async server")
	}
	return &AppConfigAsyncServer{
		client: client,
		serf:   serf,
		nodeId: nodeId,
	}, nil
}

func (c *AppConfigAsyncServer) Serve() {
	err := c.client.ReceiveConfig(func(orgId, namespaceName, appName, seccompProfile, strategy string, quotas map[string]float64) error {
		cmd := fmt.Sprintf("Organization: %s\nNamespace: %s\nApplication: %s\nSeccomp profile: %s\nResource quotas:\n", orgId, namespaceName, appName, seccompProfile)
		for resource, quota := range quotas {
			cmd += fmt.Sprintf("\t%s: %f\n", resource, quota)
		}
		log.Println(cmd)
		eventName := fmt.Sprintf("app_config-%s-%v", c.nodeId, time.Now().Unix())
		return c.serf.TriggerUserEvent(eventName, cmd, true)
	})
	if err != nil {
		log.Println(err)
	}
}

func (c *AppConfigAsyncServer) GracefulStop() {
	c.client.GracefulStop()
}
