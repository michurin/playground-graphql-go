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

func getLoaderFnByName(p graphql.ResolveParams, name string, key dataloader.Key) dataloader.Thunk {
	return p.Context.Value("dataloaders").(map[string]*dataloader.Loader)[name].Load(p.Context, key)
}

// ----- business objects -----

// Driver

type Driver struct {
	id int
}

func NewDriver(id int) *Driver {
	return &Driver{id: id}
}

func (r *Driver) Resolve(p graphql.ResolveParams) (interface{}, error) {
	switch p.Info.FieldName {
	case "id":
		return r.id, nil
	case "name":
		trunk := getLoaderFnByName(p, "driver", dataloader.StringKey(fmt.Sprintf("%d", r.id)))
		return func() (interface{}, error) {
			data, err := trunk()
			if err != nil {
				return nil, err
			}
			return data.(sqlite3.RowMap)["name"], nil
		}, nil
	case "rides":
		trunk := getLoaderFnByName(p, "rides_by_driver_id",
			dataloader.StringKey(fmt.Sprintf("%d", r.id)),
		)
		return func() (interface{}, error) {
			data, err := trunk()
			if err != nil {
				return nil, err
			}
			dataArray := data.([]sqlite3.RowMap)
			r := make([]*CompleteRide, len(dataArray))
			for i, e := range dataArray {
				r[i] = &CompleteRide{
					Id:          int(e["ride_id"].(int64)),
					Driver:      NewDriver(int(e["driver_id"].(int64))),
					Customer:    NewCustomer(int(e["customer_id"].(int64))),
					Destination: e["destination"].(string),
				}
			}
			return r, nil
		}, nil
	}
	return nil, errors.New("Unknown field " + p.Info.FieldName)
}

// Customer

type Customer struct {
	id int
}

func (r *Customer) Resolve(p graphql.ResolveParams) (interface{}, error) {
	switch p.Info.FieldName {
	case "id":
		return r.id, nil
	case "name":
		trunk := getLoaderFnByName(p, "customer", dataloader.StringKey(fmt.Sprintf("%d", r.id)))
		return func() (interface{}, error) {
			data, err := trunk()
			if err != nil {
				return nil, err
			}
			return data.(sqlite3.RowMap)["name"], nil
		}, nil
	case "rides":
		trunk := getLoaderFnByName(p, "rides_by_customer_id", // Only one change: name of loader
			dataloader.StringKey(fmt.Sprintf("%d", r.id)),
		)
		return func() (interface{}, error) {
			data, err := trunk()
			if err != nil {
				return nil, err
			}
			dataArray := data.([]sqlite3.RowMap)
			r := make([]*CompleteRide, len(dataArray))
			for i, e := range dataArray {
				r[i] = &CompleteRide{
					Id:          int(e["ride_id"].(int64)),
					Driver:      NewDriver(int(e["driver_id"].(int64))),
					Customer:    NewCustomer(int(e["customer_id"].(int64))),
					Destination: e["destination"].(string),
				}
			}
			return r, nil
		}, nil
	}
	return nil, errors.New("Unknown field " + p.Info.FieldName)
}

func NewCustomer(id int) *Customer {
	return &Customer{id: id}
}

// Ride

type Ride struct {
	id    int
	trunk dataloader.Thunk
}

func (r *Ride) getTrunk(p graphql.ResolveParams) dataloader.Thunk {
	if r.trunk == nil {
		r.trunk = getLoaderFnByName(p, "ride", dataloader.StringKey(fmt.Sprintf("%d", r.id)))
	}
	return r.trunk
}

func (r *Ride) Resolve(p graphql.ResolveParams) (interface{}, error) {
	switch p.Info.FieldName {
	case "id":
		return r.id, nil
	case "driver":
		trunk := r.getTrunk(p)
		return func() (interface{}, error) {
			data, err := trunk()
			if err != nil {
				return nil, err
			}
			return NewDriver(int(data.(sqlite3.RowMap)["driver_id"].(int64))), nil
		}, nil
	case "customer":
		trunk := r.getTrunk(p)
		return func() (interface{}, error) {
			data, err := trunk()
			if err != nil {
				return nil, err
			}
			return NewCustomer(int(data.(sqlite3.RowMap)["customer_id"].(int64))), nil
		}, nil
	case "destination":
		trunk := r.getTrunk(p)
		return func() (interface{}, error) {
			data, err := trunk()
			if err != nil {
				return nil, err
			}
			return data.(sqlite3.RowMap)["destination"], nil
		}, nil
	}
	return nil, errors.New("Unknown field " + p.Info.FieldName)
}

func NewRide(id int) *Ride {
	return &Ride{id: id}
}

// Ride: completely resolved

type CompleteRide struct {
	Id          int
	Driver      *Driver
	Customer    *Customer
	Destination string
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
		"ride": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			var results []*dataloader.Result
			keysString := make([]string, len(keys))
			for idx, e := range keys {
				keysString[idx] = e.String()
			}
			res := sql(fmt.Sprintf("select * from Ride where ride_id in (%s)", strings.Join(keysString, ", "))) // Oh. Invalid request if empty list
			data := map[int]sqlite3.RowMap{}
			for _, e := range res {
				data[int(e["ride_id"].(int64))] = e
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
	var driverType = graphql.NewObject(graphql.ObjectConfig{
		Name: "driver", // used by graphlql-relay
		Fields: graphql.Fields{
			"id":   &graphql.Field{Type: graphql.Int},
			"name": &graphql.Field{Type: graphql.String},
		},
	})

	var customerType = graphql.NewObject(graphql.ObjectConfig{
		Name: "customer",
		Fields: graphql.Fields{
			"id":   &graphql.Field{Type: graphql.Int},
			"name": &graphql.Field{Type: graphql.String},
		},
	})

	var rideType = graphql.NewObject(graphql.ObjectConfig{
		Name: "ride",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.Int},
			"driver":      &graphql.Field{Type: driverType},
			"customer":    &graphql.Field{Type: customerType},
			"destination": &graphql.Field{Type: graphql.String},
		},
	})

	customerType.AddFieldConfig("rides", &graphql.Field{Type: graphql.NewList(rideType)})
	driverType.AddFieldConfig("rides", &graphql.Field{Type: graphql.NewList(rideType)})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "RootQuery",
		Fields: graphql.Fields{
			"x_ride": &graphql.Field{
				Name: "ride",
				Type: rideType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					rideId := p.Args["id"].(int)
					return NewRide(rideId), nil
				},
			},
			"x_customer": &graphql.Field{
				Type: customerType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					customerId := p.Args["id"].(int)
					return NewCustomer(customerId), nil
				},
			},
		},
	})

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"add_ride": &graphql.Field{
				Name: "add_ride",
				Type: rideType,
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
					customerId := p.Args["customer_id"].(int)
					driverId := p.Args["driver_id"].(int)
					destination := p.Args["destination"].(string)
					res = sql(fmt.Sprintf(
						"insert into Ride (ride_id, customer_id, driver_id, destination) values (%d, %d, %d, \"%s\")",
						nextRideId,
						customerId,
						driverId,
						destination,
					))
					res = sql(fmt.Sprintf(
						"select * from Ride where ride_id=%d",
						nextRideId,
					))
					return &CompleteRide{
						Id:          int(nextRideId),
						Driver:      NewDriver(driverId),
						Customer:    NewCustomer(customerId),
						Destination: destination,
					}, nil
				},
			},
		},
	})

	var schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
		// Types: []graphql.Type{customerType}, // ??
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
