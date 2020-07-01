package healthcheck

import (
	"strings"
)

func topology(nodeid string) string {
	return strings.ReplaceAll(nodeid, ".", "/")
}

func used(nodeid string) bool {
	return strings.Contains(nodeid, ".")
}
