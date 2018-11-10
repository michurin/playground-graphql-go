package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
)

type Node struct {
	Name string
}

func (n Node) Resolve(p graphql.ResolveParams) (interface{}, error) {
	switch p.Info.FieldName {
	case "nm":
		return n.Name, nil
	}
	return nil, errors.New("Node resolver: Unknown field " + p.Info.FieldName)
}

func main() {

	var nodeType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Node",
		Fields: graphql.Fields{
			"nm": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Name: "a",
				Type: nodeType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return Node{"Some constant title"}, nil
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
