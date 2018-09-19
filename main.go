package main

import (
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/mxk/go-sqlite/sqlite3"
	gqlhandler "github.com/graphql-go/graphql-go-handler"
	"net/http"
	"errors"
)

// TODO:
// + custom types
// + arguments
// + args from parent nodes (use .source inst)
// + cache?.. recursion?..
// - make shared DB adapter

// ----- draft sql interface -----

func sql(sql string) []sqlite3.RowMap {
	var result []sqlite3.RowMap
	c, err := sqlite3.Open("database.db")
	if err != nil {
		panic(err)
	}
	s, err := c.Query(sql)
	if err != nil {
		panic(sql + " -> " + err.Error())
	}
	for {
		row := make(sqlite3.RowMap)
		s.Scan(row)
		result = append(result, row)
		err = s.Next()
		if err != nil {
			break
		}
	}
	fmt.Printf("SQL: %s -> %v\n", sql, result)
	return result
}

// ----- types -----

var DriverType = graphql.NewObject(graphql.ObjectConfig{
	Name: "driver",
	Fields: graphql.Fields{
		"id":   &graphql.Field{
			Type: graphql.Int,
			Resolve: func (p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["driver_id"], nil
			},
		},
		"name": &graphql.Field{
			Type: graphql.String,
			Resolve: func (p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["name"], nil
			},
		},
	},
})

var CustomerType = graphql.NewObject(graphql.ObjectConfig{
	Name: "customer",
	Fields: graphql.Fields{
		"id": &graphql.Field{
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["customer_id"], nil
			},
		},
		"name": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["name"], nil
			},
		},
	},
})

var RideType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ride",
	Fields: graphql.Fields{
		"id":   &graphql.Field{
			Type: graphql.Int,
			Resolve: func (p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["ride_id"], nil
			},
		},
		"driver": &graphql.Field{
			Type: DriverType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				driverId := p.Source.(sqlite3.RowMap)["driver_id"]
				fmt.Println("Call for driver", driverId)
				res :=  sql(fmt.Sprintf("select * from Driver where driver_id=%d", driverId))
				if len(res) == 0 {
					return nil, nil
				}
				// We return sqlite3.RowMap asis
				// GrQL engine pass this result asis to Type resolver
				return res[0], nil
			},
		},
		"customer": &graphql.Field{
			Type: CustomerType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				customerId := p.Source.(sqlite3.RowMap)["customer_id"]
				fmt.Println("Call for customer", customerId)
				res :=  sql(fmt.Sprintf("select * from Customer where customer_id=%d", customerId))
				if len(res) == 0 {
					return nil, nil
				}
				return res[0], nil
			},
		},
		"destination": &graphql.Field{
			Type: graphql.String,
			Resolve: func (p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["destination"], nil
			},
		},
	},
})

// ----- m.a.i.n -----

func main() {
	fields := graphql.Fields{
		"x_ride": &graphql.Field{
			Name: "ride",
			Type: RideType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				rideId := p.Args["id"] // TODO: or from context?!
				if rideId == nil {
					return nil, errors.New("id arg required")
				}
				res :=  sql(fmt.Sprintf("select * from Ride where ride_id=%d", rideId))
				if len(res) == 0 {
					return nil, nil
				}
				return res[0], nil
			},
		},
		/*
		"x_customer": &graphql.Field{
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
				return Customer{int(res[0]["customer_id"].(int64)), res[0]["name"].(string)}, nil
			},
		},
		*/
	}
	rootQuery := graphql.ObjectConfig{
		Name: "RootQuery",
		Fields: fields,
	}

	var schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(rootQuery),
		//Types: []graphql.Type{CustomerType}, // ??
	})
	if err != nil {
		panic(err)
	}

	//fmt.Printf("%#v\n", schema.TypeMap())

	handler := gqlhandler.New(&gqlhandler.Config{
		Schema: &schema,
		Pretty: true,
	})
	http.Handle("/gql", handler)
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_ride(id:2) {id destination customer {id name} driver {id name}} }'")
	http.ListenAndServe(":8080", nil)
}
