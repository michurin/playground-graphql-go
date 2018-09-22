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
		trunk := getLoaderFnByName(p, "driver", NewIntKey(r.id))
		return func() (interface{}, error) {
			data, err := trunk()
			if err != nil {
				return nil, err
			}
			return data.(sqlite3.RowMap)["name"], nil
		}, nil
	case "rides":
		trunk := getLoaderFnByName(p, "rides_by_driver_id", NewIntKey(r.id))
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
		trunk := getLoaderFnByName(p, "customer", NewIntKey(r.id))
		return func() (interface{}, error) {
			data, err := trunk()
			if err != nil {
				return nil, err
			}
			return data.(sqlite3.RowMap)["name"], nil
		}, nil
	case "rides":
		trunk := getLoaderFnByName(p, "rides_by_customer_id", NewIntKey(r.id)) // Only one change: name of loader
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
		r.trunk = getLoaderFnByName(p, "ride", NewIntKey(r.id))
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

// Key interface (in fact, dataloader uses .String() as key)

type IntKey struct {
	raw int
	str string
}

func (k IntKey) String() string { return k.str }

func (k IntKey) Raw() interface{} { return k.raw }

func NewIntKey(i int) IntKey {
	return IntKey{
		raw: i,
		str: strconv.Itoa(i),
	}
}

// Collection of loaders

func keysToString(keys dataloader.Keys) string {
	keysString := make([]string, len(keys))
	for idx, e := range keys {
		keysString[idx] = e.String()
	}
	return strings.Join(keysString, ", ")
}

func loadOneToOne(sqlTemplate string, keyField string, keys dataloader.Keys) []*dataloader.Result {
	var results []*dataloader.Result
	res := sql(fmt.Sprintf(sqlTemplate, keysToString(keys))) // Oh. Invalid request if empty list
	data := map[int]sqlite3.RowMap{}
	for _, e := range res {
		data[int(e[keyField].(int64))] = e
	}
	for _, e := range keys {
		d := data[e.Raw().(int)]
		results = append(results, &dataloader.Result{d, nil}) // TODO we can put errors here
	}
	return results
}

func loadOneToMany(sqlTemplate string, keyField string, keys dataloader.Keys) []*dataloader.Result {
	var results []*dataloader.Result
	res := sql(fmt.Sprintf(sqlTemplate, keysToString(keys))) // Oh. Invalid request if empty list
	if len(res) == 0 {
		return nil
	}
	data := map[int][]sqlite3.RowMap{}
	for _, e := range res {
		i := int(e[keyField].(int64))
		data[i] = append(data[i], e)
	}
	for _, e := range keys {
		d := data[e.Raw().(int)]
		results = append(results, &dataloader.Result{d, nil}) // TODO we can put errors here
	}
	return results
}

func NewLoaders() map[string](*dataloader.Loader) {
	// we can do here all per-request stuff
	fmt.Println("Loaders created")
	return map[string]*dataloader.Loader{
		"driver": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			return loadOneToOne("select * from Driver where driver_id in (%s)", "driver_id", keys)
		}),
		"customer": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			return loadOneToOne("select * from Customer where customer_id in (%s)", "customer_id", keys)
		}),
		"ride": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			return loadOneToOne("select * from Ride where ride_id in (%s)", "ride_id", keys)
		}),
		"rides_by_customer_id": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			return loadOneToMany("select * from Ride where customer_id in (%s)", "customer_id", keys)
		}),
		"rides_by_driver_id": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			return loadOneToMany("select * from Ride where driver_id in (%s)", "driver_id", keys)
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
						Type: graphql.Int, // TODO: write variant with graphql.NewList(graphql.Int)
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					customerId := p.Args["id"].(int)
					return NewCustomer(customerId), nil
				},
			},
		},
	})

	mutationParamsType := graphql.NewNonNull(graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "complex", // MUST?
		Fields: graphql.InputObjectConfigFieldMap{
			"customer_id": &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
			"driver_id": &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
			"destination": &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.String),
			},
		},
	}))

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"add_ride": &graphql.Field{
				Name: "add_ride",
				Type: rideType,
				Args: graphql.FieldConfigArgument{
					"params": &graphql.ArgumentConfig{Type: mutationParamsType},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					params := p.Args["params"].(map[string]interface{})
					customerId := params["customer_id"].(int)
					driverId := params["driver_id"].(int)
					destination := params["destination"].(string)
					// Oh. Just POC. Very (very!) bad code.
					// We just use sqlite backend to emulate abstract microservice or something else
					res := sql("select max(ride_id) max_ride_id from Ride")
					nextRideId := int(res[0]["max_ride_id"].(int64)) + 1
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
						Id:          nextRideId,
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
	fmt.Println("curl -XPOST http://localhost:8080/gql -H 'Content-Type: applicationgraphql' -d 'mutation { add_ride(params:{customer_id:100 driver_id:1 destination:\"One\"}){id, customer{name}} }'")
	fmt.Println()
	http.ListenAndServe(":8080", nil)
}
