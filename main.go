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

type Ride struct {
	Id   int
	Driver Driver
}

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

var RideType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Ride",
	Fields: graphql.Fields{
		"id":   &graphql.Field{Type: graphql.Int},
		"driver": &graphql.Field{
			Type: DriverType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				fmt.Printf("driver resolver source: %+v\n", p.Source)
				return Driver{10, "Ten"}, nil
			},
		},
	},
})

func main() {
	fields := graphql.Fields{
		"lastRide": &graphql.Field{
			Type: RideType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				fmt.Printf("lastRide resolver params: %+v\n", p.Args)
				return Ride{1, Driver{100, ""}}, nil
			},
		},
	}
	rootQuery := graphql.ObjectConfig{
		Name: "RootQuery",
		Fields: fields,
	}

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
