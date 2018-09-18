package main

import (
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/mxk/go-sqlite/sqlite3"
	gqlhandler "github.com/graphql-go/graphql-go-handler"
	"net/http"
)

// TODO:
// + custom types
// + arguments
// - args from parent nodes
// - cache?.. recursion?..
// - make shared DB adapter

func sql(sql string) []sqlite3.RowMap {
	var result []sqlite3.RowMap
	c, err := sqlite3.Open("database.db")
	if err != nil {
		panic(err)
	}
	s, err := c.Query(sql)
	if err != nil {
		return result
	}
	for {
		row := make(sqlite3.RowMap)
		s.Scan(row)
		result = append(result, row)
		fmt.Printf("%#v\n", row)
		err = s.Next()
		if err != nil {
			break
		}
	}
	fmt.Println(result)
	return result
}

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


type Customer struct { // TODO: add rides!
	Id int
	Name string
}

var CustomerType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Customer",
	Fields: graphql.Fields{
		"id": &graphql.Field{Type: graphql.Int},
		"name": &graphql.Field{Type: graphql.String},
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
		"customer": &graphql.Field{
			Type: CustomerType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				customerId := p.Args["id"] // TODO: or from context?!
				res :=  sql(fmt.Sprintf("select * from Customer where customer_id=%d", customerId))
				fmt.Printf("customer resolver params: %+v\n", p.Args)
				fmt.Printf("%+v\n", res)
				if len(res) == 0 {
					return nil, nil
				}
				fmt.Printf("type=%T\n", res[0]["customer_id"])
				return Customer{int(res[0]["customer_id"].(int64)), res[0]["name"].(string)}, nil
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
