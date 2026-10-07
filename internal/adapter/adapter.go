package adapter

import (
	"github.com/anton-kaitharan/wardent/internal/event"
)

type Adapter interface {
	AgentKind() string
	Parse(eventName string, raw []byte) (event.AgentEvent, error)
}
