package healthcheck

import (
	"context"
)

type Healthchecker interface {
	Start(ctx context.Context)
	UpdateId(newId string)
}
