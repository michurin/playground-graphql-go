package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/graph-gophers/dataloader"
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
// + LoadMany and rides-by-driver like requests using dataLoader
// + handler wrapper to force context and headers

// ----- draft sql interface -----

func sql(sql string) []sqlite3.RowMap {
	// fmt.Println(sql)
	var result []sqlite3.RowMap
	c, err := sqlite3.Open("database.db")
	if err != nil {
		panic(err)
	}
	s, err := c.Query(sql)
	if err == nil {
		for {
			row := make(sqlite3.RowMap)
			s.Scan(row)
			result = append(result, row)
			err = s.Next()
			if err != nil {
				break
			}
		}
	}
	c.Commit()
	c.Close()
	fmt.Printf("SQL: %s -> %v\n", sql, result)
	return result
}

// ----- http -----

type gtHandler struct {
	origHandler http.Handler
}

func (h *gtHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("X-Michurin", "Here!")
	// TODO We will create loader on mutations too. Is it ok?
	h.origHandler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "dataloaders", NewLoaders())))
}

func handlerWrapper(h http.Handler) *gtHandler {
	return &gtHandler{h}
}

// ----- util -----

func getLoaderFnByName(p graphql.ResolveParams, name string, key dataloader.Key) func() (interface{}, error) {
	return p.Context.Value("dataloaders").(map[string]*dataloader.Loader)[name].Load(p.Context, key)
}

func loaderFn(p graphql.ResolveParams, key dataloader.Key) func() (interface{}, error) {
	return getLoaderFnByName(p, p.Info.FieldName, key)
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
				// The same effect
				//name := p.Source.(sqlite3.RowMap)["name"]
				//fmt.Printf("INFO: %#v\n", p.Info.RootValue)
				//fmt.Println("[FR] Prepare function-result for", name)
				//return func() (interface{}, error) {
				//	fmt.Println("[FR] Call function-result for", name)
				//	return name, nil
				//}, nil
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
				fn := getLoaderFnByName(p, "rides_by_customer_id",
					dataloader.StringKey(fmt.Sprintf("%d", customerId)),
				)
				return fn, nil
			},
		},
	)
	DriverType.AddFieldConfig(
		"rides",
		&graphql.Field{ // synthetic field
			Type: graphql.NewList(RideType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				driverId := p.Source.(sqlite3.RowMap)["driver_id"]
				fn := getLoaderFnByName(p, "rides_by_driver_id",
					dataloader.StringKey(fmt.Sprintf("%d", driverId)),
				)
				return fn, nil
			},
		},
	)
}

// ----- loaders -----

func NewLoaders() map[string](*dataloader.Loader) {
	// we can do here all per-request stuff
	fmt.Println("Loaders created")
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
				data[int(e["customer_id"].(int64))] = e
			}
			for _, e := range keys {
				r := e.String() // TODO use e.Raw().cast
				i, _ := strconv.Atoi(r)
				d := data[i]
				results = append(results, &dataloader.Result{d, nil}) // TODO we can put errors here
			}
			return results
		}),
		"rides_by_customer_id": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			var results []*dataloader.Result
			keysString := make([]string, len(keys))
			for idx, e := range keys {
				keysString[idx] = e.String()
			}
			res := sql(fmt.Sprintf("select * from Ride where customer_id in (%s)", strings.Join(keysString, ", "))) // Oh. Invalid request if empty list
			if len(res) == 0 {
				return nil
			}
			data := map[int][]sqlite3.RowMap{}
			for _, e := range res {
				i := int(e["customer_id"].(int64))
				data[i] = append(data[i], e)
			}
			for _, e := range keys {
				r := e.String() // TODO use e.Raw().cast
				i, _ := strconv.Atoi(r)
				d := data[i]
				results = append(results, &dataloader.Result{d, nil}) // TODO we can put errors here
			}
			return results
		}),
		"rides_by_driver_id": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			var results []*dataloader.Result
			keysString := make([]string, len(keys))
			for idx, e := range keys {
				keysString[idx] = e.String()
			}
			res := sql(fmt.Sprintf("select * from Ride where driver_id in (%s)", strings.Join(keysString, ", "))) // Oh. Invalid request if empty list
			if len(res) == 0 {
				return nil
			}
			data := map[int][]sqlite3.RowMap{}
			for _, e := range res {
				i := int(e["driver_id"].(int64))
				data[i] = append(data[i], e)
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
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "RootQuery",
		Fields: graphql.Fields{
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
		},
	})
	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"add_ride": &graphql.Field{
				Name: "add_ride",
				Type: RideType,
				Args: graphql.FieldConfigArgument{
					"customer_id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
					"driver_id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
					"destination": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					// Oh. Just POC. Very (very!) bad code
					res := sql("select max(ride_id) max_ride_id from Ride")
					nextRideId := res[0]["max_ride_id"].(int64) + 1
					fmt.Println("next", nextRideId)
					res = sql(fmt.Sprintf(
						"insert into Ride (ride_id, customer_id, driver_id, destination) values (%d, %d, %d, \"%s\")",
						nextRideId,
						p.Args["customer_id"].(int),
						p.Args["driver_id"].(int),
						p.Args["destination"].(string),
					))
					res = sql(fmt.Sprintf(
						"select * from Ride where ride_id=%d",
						nextRideId,
					))
					return res[0], nil // return res for compat
				},
			},
		},
	})
	var schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
		// Types: []graphql.Type{CustomerType}, // ??
	})
	if err != nil {
		panic(err)
	}

	//fmt.Printf("%#v\n", schema.TypeMap())

	handler := handlerWrapper(handler.New(&handler.Config{
		Schema: &schema,
		Pretty: true,
	}))
	http.Handle("/gql", handler)

	fmt.Println("Examples:")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_ride(id:2) {id destination customer {id name} driver {id name}} }'")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_customer(id: 200) {id name, rides {id, destination, driver {name}}} }'")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_ride(id: 3) {id destination customer {id name rides {id driver {name}}}} }'")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_ride(id: 3) {id destination customer {id name rides {id driver {name rides {id}}}}} }")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'query { x_customer(id: 200) {rides{ driver{rides{ driver{rides{ driver{name} }} }} }} }'")
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d 'mutation { add_ride(customer_id:200 driver_id:1 destination:\"Home\"){id, customer{name}} }'")
	fmt.Println()
	http.ListenAndServe(":8080", nil)
}
