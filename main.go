package main

import (
	"fmt"
	"net/http"

	"github.com/graphql-go/graphql"
	gqlhandler "github.com/graphql-go/graphql-go-handler"
)

// TODO:
// + custom types
// + arguments
// - args from parent nodes
// - cache?.. recursion?..

type Driver struct {
	Id   int
	Name string
}

var DriverType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Driver",
	Fields: graphql.Fields{
		"id":   &graphql.Field{Type: graphql.Int},
		"name": &graphql.Field{Type: graphql.String},
	},
})

func main() {
	fields := graphql.Fields{
		"lastRide": &graphql.Field{
			Type: DriverType,
			Args: graphql.FieldConfigArgument{
				"x": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				fmt.Printf("Handler params: %+v\n", p.Args)
				return Driver{1, "Me"}, nil
			},
		},
	}
	rootQuery := graphql.ObjectConfig{Name: "RootQuery", Fields: fields}

	var schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(rootQuery),
	})
	if err != nil {
		panic(err)
	}

	handler := gqlhandler.New(&gqlhandler.Config{
		Schema: &schema,
		Pretty: true,
	})
	http.Handle("/gql", handler)
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query Root{ lastRide }'")
	http.ListenAndServe(":8080", nil)
}
