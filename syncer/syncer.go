package syncer

// import fPb "github.com/c12s/scheme/flusher"

type Fn func(event []byte)

type Syncer interface {
	Subscribe(topic string, f Fn)
	Alter() error
	Error(topic string, data []byte)
}
