package query

import (
	"context"
	"fmt"
	"perch/internal/hummingbird"
	fixtures "perch/testdata/hummingbird"
	"time"
)

type Node = hummingbird.Node
type nodeBackend interface {
	Nodes(context.Context) ([]Node, error)
}

func (s Service) nodes(ctx context.Context) (Response, error) {
	out := Response{SchemaVersion: 1, Source: "hummingbird"}
	var rows []Node
	var err error
	if s.Demo {
		out.Source = "demo"
		rows, err = hummingbird.ParseNodes(fixtures.Nodes)
	} else {
		b, ok := s.Backend.(nodeBackend)
		if !ok {
			return out, fmt.Errorf("node allocations unavailable on this backend")
		}
		rows, err = b.Nodes(ctx)
	}
	if err != nil {
		return out, err
	}
	out.At = time.Now()
	out.Data = rows
	return out, nil
}
