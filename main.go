package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/graph-gophers/dataloader" // HOWTO: (1) update Context, (2) force headers?
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/mxk/go-sqlite/sqlite3"
)

// TODO:
// + custom types
// + arguments
// + args from parent nodes (use .source inst)
// + cache?.. recursion?..
// + handle start of request and fill context
// + use dataloader
// - LoadMany and rides-by-driver like requests using dataLoader
// - handler wrapper to force context and headers

// ----- draft sql interface -----

func sql(sql string) []sqlite3.RowMap {
	// fmt.Println(sql)
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

// ----- util -----

func loaderFn(p graphql.ResolveParams, key dataloader.Key) (func () (interface{}, error)) {
	// TODO use Context inst RootValue?
	return p.Info.RootValue.(map[string]interface{})["dataloaders"].(map[string]*dataloader.Loader)[p.Info.FieldName].Load(
		p.Context,
		key,
	)
}

// ----- types -----

var DriverType = graphql.NewObject(graphql.ObjectConfig{
	Name: "driver", // used by graphlql-relay
	Fields: graphql.Fields{
		"id": &graphql.Field{
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["driver_id"], nil
			},
		},
		"name": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["name"], nil
				/*
				// The same
				name := p.Source.(sqlite3.RowMap)["name"]
				fmt.Printf("INFO: %#v\n", p.Info.RootValue)
				fmt.Println("[FR] Prepare function-result for", name)
				return func() (interface{}, error) {
					fmt.Println("[FR] Call function-result for", name)
					return name, nil
				}, nil
				*/
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
		"id": &graphql.Field{
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["ride_id"], nil
			},
		},
		"driver": &graphql.Field{
			Type: DriverType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				driverId := p.Source.(sqlite3.RowMap)["driver_id"]
				return loaderFn(p, dataloader.StringKey(fmt.Sprintf("%d", driverId))), nil
			},
		},
		"customer": &graphql.Field{
			Type: CustomerType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				customerId := p.Source.(sqlite3.RowMap)["customer_id"]
				return loaderFn(p, dataloader.StringKey(fmt.Sprintf("%d", customerId))), nil
			},
		},
		"destination": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return p.Source.(sqlite3.RowMap)["destination"], nil
			},
		},
	},
})

func init() {
	CustomerType.AddFieldConfig(
		"rides",
		&graphql.Field{ // synthetic field
			Type: graphql.NewList(RideType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				customerId := p.Source.(sqlite3.RowMap)["customer_id"]
				res := sql(fmt.Sprintf("select * from Ride where customer_id=%d", customerId))
				if len(res) == 0 {
					return nil, nil
				}
				return res, nil
			},
		},
	)
	DriverType.AddFieldConfig(
		"rides",
		&graphql.Field{ // synthetic field
			Type: graphql.NewList(RideType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				driverId := p.Source.(sqlite3.RowMap)["driver_id"]
				res := sql(fmt.Sprintf("select * from Ride where driver_id=%d", driverId))
				if len(res) == 0 {
					return nil, nil
				}
				return res, nil
			},
		},
	)
}

// ----- loaders -----

func NewLoaders () map[string](*dataloader.Loader) {
	// we can do here all per-request stuff
	return map[string]*dataloader.Loader{
		"driver": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			var results []*dataloader.Result
			keysString := make([]string, len(keys))
			for idx, e := range keys {
				keysString[idx] = e.String()
			}
			res := sql(fmt.Sprintf("select * from Driver where driver_id in (%s)", strings.Join(keysString, ", "))) // Oh. Invalid request if empty list
			data := map[int]sqlite3.RowMap{}
			for _, e := range res {
				data[int(e["driver_id"].(int64))] = e
			}
			for _, e := range keys {
				r := e.String() // TODO use e.Raw().cast
				i, _ := strconv.Atoi(r)
				d := data[i]
				results = append(results, &dataloader.Result{d, nil}) // TODO we can put errors here
			}
			return results
		}),
		"customer": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			var results []*dataloader.Result
			keysString := make([]string, len(keys))
			for idx, e := range keys {
				keysString[idx] = e.String()
			}
			res := sql(fmt.Sprintf("select * from Customer where customer_id in (%s)", strings.Join(keysString, ", "))) // Oh. Invalid request if empty list
			data := map[int]sqlite3.RowMap{}
			for _, e := range res {
				data[int(e["driver_id"].(int64))] = e
			}
			for _, e := range keys {
				r := e.String() // TODO use e.Raw().cast
				i, _ := strconv.Atoi(r)
				d := data[i]
				results = append(results, &dataloader.Result{d, nil}) // TODO we can put errors here
			}
			return results
		}),
	}
}

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
				rideId := p.Args["id"]
				if rideId == nil {
					return nil, errors.New("id arg required")
				}
				res := sql(fmt.Sprintf("select * from Ride where ride_id=%d", rideId))
				if len(res) == 0 {
					return nil, nil
				}
				return res[0], nil
			},
		},
		"x_customer": &graphql.Field{
			Type: CustomerType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				customerId := p.Args["id"]
				if customerId == nil {
					return nil, errors.New("id arg required")
				}
				res := sql(fmt.Sprintf("select * from Customer where customer_id=%d", customerId))
				if len(res) == 0 {
					return nil, nil
				}
				return res[0], nil
			},
		},
	}
	rootQuery := graphql.ObjectConfig{
		Name:   "RootQuery",
		Fields: fields,
	}

	var schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(rootQuery),
		// Types: []graphql.Type{CustomerType}, // ??
	})
	if err != nil {
		panic(err)
	}

	//fmt.Printf("%#v\n", schema.TypeMap())

	handler := handler.New(&handler.Config{
		Schema: &schema,
		Pretty: true,
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} { // POC init req
			return map[string]interface{}{"dataloaders": NewLoaders()}
		},
	})
	http.Handle("/gql", handler)

	fmt.Println("Examples:")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_ride(id:2) {id destination customer {id name} driver {id name}} }'")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_customer(id: 200) {id name, rides {id, destination, driver {name}}} }'")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_ride(id: 3) {id destination customer {id name rides {id driver {name}}}} }'")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_ride(id: 3) {id destination customer {id name rides {id driver {name rides {id}}}}} }")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_customer(id: 200) {rides{ driver{rides{ driver{rides{ driver{name} }} }} }} }'")

	http.ListenAndServe(":8080", nil)
}
