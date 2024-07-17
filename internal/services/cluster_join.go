package services

import (
	"fmt"
	"log"
	"strings"

	"github.com/c12s/star/internal/domain"
	"github.com/nats-io/nats.go"
)

type ClusterJoinListener struct {
	conn        *nats.Conn
	serf        *SerfAgent
	nodeId      string
	nodeIdStore domain.NodeIdStore
}

func NewClusterJoinListener(conn *nats.Conn, serf *SerfAgent, nodeId string, nodeIdStore domain.NodeIdStore) *ClusterJoinListener {
	return &ClusterJoinListener{
		conn:        conn,
		serf:        serf,
		nodeId:      nodeId,
		nodeIdStore: nodeIdStore,
	}
}

func (l *ClusterJoinListener) Listen() {
	_, err := l.conn.Subscribe(fmt.Sprintf("%s.join", l.nodeId), func(msg *nats.Msg) {
		params := strings.Split(string(msg.Data), "|")
		if len(params) != 2 {
			log.Println("invalid cluster join params: ", params)
			return
		}
		address := params[0]
		clusterId := params[1]
		// todo: what happens if the node is not a member of a cluster?
		//l.serf.Leave()
		err := l.serf.Join(address)
		if err != nil {
			log.Println(err)
			return
		}
		err = l.nodeIdStore.PutClusterId(clusterId)
		if err != nil {
			log.Println(err)
		}
	})
	if err != nil {
		log.Println(err)
	}
}
