package main

import (
	"fmt"
	"net/http"

	"github.com/graphql-go/graphql"
	gqlhandler "github.com/graphql-go/graphql-go-handler"
)

// TODO:
// - custom types
// - arguments
// - cache?.. recursion?..

var queryType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		"lastRide": &graphql.Field{
			Type: graphql.Int,
			Resolve: func(args graphql.ResolveParams) (interface{}, error) {
				fmt.Printf("Handler args: %#v\n", args)
				return 1, nil
			},
		},
	},
})

var Schema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: queryType,
})

func main() {
	handler := gqlhandler.New(&gqlhandler.Config{
		Schema: &Schema,
		Pretty: true,
	})
	http.Handle("/gql", handler)
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query Root{ lastRide }'")
	http.ListenAndServe(":8080", nil)
}
