package healthcheck

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	nats "github.com/nats-io/nats.go"
	"time"
)

type Healthcheck struct {
	natsConnection *nats.Conn
	nodeId         string
	topic          string
	t              time.Duration
	hostParams     bool
	labels         map[string]string
}

func New(address, topic, nodeid, period string, labels map[string]string) (*Healthcheck, error) {
	natsConnection, err := nats.Connect(address)
	if err != nil {
		return nil, err
	}

	t, err := time.ParseDuration(period)
	if err != nil {
		return nil, err
	}

	return &Healthcheck{
		natsConnection: natsConnection,
		nodeId:         nodeid,
		topic:          topic,
		t:              t,
		hostParams:     true,
		labels:         labels,
	}, nil
}

func (h *Healthcheck) push() error {
	data, err := h.metrics()
	if err != nil {
		return err
	}

	state, err := proto.Marshal(data)
	if err != nil {
		return err
	}
	h.natsConnection.Publish(h.topic, state)
	err = h.natsConnection.Flush()
	if err == nil && h.hostParams {
		h.hostParams = false // after first send, do not send it anymore unless machine is restarted or updated or something
	}

	return nil
}

func (h *Healthcheck) UpdateId(newId string) {
	h.nodeId = newId
}

func (h *Healthcheck) Start(ctx context.Context) {
	ticker := time.NewTicker(h.t)
	for {
		select {
		case <-ctx.Done():
			fmt.Println("healthcheck done")
			return
		case <-ticker.C:
			err := h.push()
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			fmt.Println("Data pushed")
		}
	}
}
