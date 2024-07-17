package servers

import (
	"errors"
	"fmt"
	"log"
	"time"

	kuiperapi "github.com/c12s/kuiper/pkg/api"
	"github.com/c12s/star/internal/domain"
	proto_mapper "github.com/c12s/star/internal/mappers/proto"
	"github.com/c12s/star/internal/services"
	"google.golang.org/protobuf/proto"
)

type ConfigAsyncServer struct {
	client  *kuiperapi.KuiperAsyncClient
	configs domain.ConfigStore
	serf    *services.SerfAgent
	nodeId  string
}

func NewConfigAsyncServer(client *kuiperapi.KuiperAsyncClient, configs domain.ConfigStore, serf *services.SerfAgent, nodeId string) (*ConfigAsyncServer, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	return &ConfigAsyncServer{
		client:  client,
		configs: configs,
		serf:    serf,
		nodeId:  nodeId,
	}, nil
}

func (c *ConfigAsyncServer) Serve() {
	err := c.client.ReceiveConfig(
		func(protoConfig *kuiperapi.StandaloneConfig, namespace, strategy string) error {
			config, err := proto_mapper.ApplyStandaloneConfigCommandToDomain(protoConfig, namespace)
			if err != nil {
				return err
			}
			putErr := c.configs.PutStandalone(config)
			if putErr != nil {
				return errors.New(putErr.Message())
			}
			if strategy == "gossip" {
				eventName := fmt.Sprintf("standalone-%s-%v", c.nodeId, time.Now().Unix())
				payload, err := proto.Marshal(protoConfig)
				if err != nil {
					return err
				}
				c.serf.TriggerUserEvent(eventName, string(payload), true)
			}
			return nil
		},
		func(protoConfig *kuiperapi.ConfigGroup, namespace, strategy string) error {
			config, err := proto_mapper.ApplyConfigGroupCommandToDomain(protoConfig, namespace)
			if err != nil {
				return err
			}
			putErr := c.configs.PutGroup(config)
			if putErr != nil {
				return errors.New(putErr.Message())
			}
			if strategy == "gossip" {
				eventName := fmt.Sprintf("group-%s-%v", c.nodeId, time.Now().Unix())
				payload, err := proto.Marshal(protoConfig)
				if err != nil {
					return err
				}
				c.serf.TriggerUserEvent(eventName, string(payload), true)
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}
}

func (c *ConfigAsyncServer) GracefulStop() {
	c.client.GracefulStop()
}
