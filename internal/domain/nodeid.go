package domain

type NodeId struct {
	Value string
}

type NodeIdStore interface {
	Get() (*NodeId, error)
	Put(nodeId NodeId) error
	PutClusterId(clusterId string) error
}
