package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
)

func main() {
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Name: "a",
				Type: graphql.Int,
				Args: graphql.FieldConfigArgument{
					"x": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					x := p.Args["x"].(int)
					return x * x, nil
				},
			},
			"b": &graphql.Field{
				Name: "b",
				Type: graphql.NewList(graphql.Int),
				Args: graphql.FieldConfigArgument{
					"x": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					x := p.Args["x"].(int)
					if x < 0 || x > 20 {
						return nil, errors.New("Invald arg")
					}
					r := make([]int, x)
					for i := 0; i < x; i++ {
						if i < 2 {
							r[i] = 1
						} else {
							r[i] = r[i-1] + r[i-2]
						}
					}
					return r, nil
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
