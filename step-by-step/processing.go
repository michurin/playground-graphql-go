package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
)

var actionSerial uint64

type Node struct {
	Name  string
	Level int
	Start uint64
	Ready uint64
	Fin   uint64
}

func (n Node) Resolve(p graphql.ResolveParams) (interface{}, error) {
	switch p.Info.FieldName {
	case "nm":
		return n.Name, nil
	case "lvl":
		return n.Level, nil
	case "seq": // h - history
		return fmt.Sprintf(" %d %d %d ", n.Start, n.Ready, n.Fin), nil
	case "sub":
		nextLevel := n.Level + 1
		nodes := make([]interface{}, 2)
		for i := 0; i < 2; i++ {
			atomic.AddUint64(&actionSerial, 1)
			nn := Node{"Subnode", nextLevel, actionSerial, 0, 0}
			ch := make(chan Node)
			go func() {
				r := rand.Intn(90) + 10
				time.Sleep(time.Duration(r) * time.Millisecond)
				atomic.AddUint64(&actionSerial, 1)
				nn.Ready = actionSerial
				ch <- nn
			}()
			nodes[i] = func() (interface{}, error) {
				n := <-ch
				atomic.AddUint64(&actionSerial, 1)
				n.Fin = actionSerial
				return n, nil
			}
		}
		return nodes, nil
	}
	return nil, errors.New("Node resolver: Unknown field " + p.Info.FieldName)
}

func main() {

	rand.Seed(time.Now().UTC().UnixNano())

	var nodeType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Node",
		Fields: graphql.Fields{
			"nm":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"lvl": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"seq": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	nodeType.AddFieldConfig("sub", &graphql.Field{
		Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(nodeType))),
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Name: "a",
				Type: nodeType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					actionSerial = 0
					return Node{"Root node", 0, 0, 0, 0}, nil
				},
			},
		},
	})

	var schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		panic(err)
	}

	handler := handler.New(&handler.Config{
		Schema:     &schema,
		Pretty:     true,
		GraphiQL:   true,
		Playground: true,
	})
	http.Handle("/gql", handler)
	fmt.Println("Run...")
	http.ListenAndServe(":8080", nil)
}
