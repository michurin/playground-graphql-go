package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/graph-gophers/dataloader"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/mxk/go-sqlite/sqlite3"
)

// ----- draft sql interface -----

const DATABASE = "database.db"

func errorString(prefix string, sql string, err error) string {
	return fmt.Sprintf("%s [%s] %s: %s", prefix, DATABASE, sql, err.Error())
}

func logResult(sql string, result []sqlite3.RowMap) {
	fmt.Printf("\x1b[1m%s\x1b[0m:\n", sql)
	for i, r := range result {
		fields := make([]string, len(r))
		j := 0
		for k, v := range r {
			fields[j] = fmt.Sprintf("%s=\x1b[1;32m%v\x1b[0m", k, v)
			j += 1
		}
		sort.Strings(fields)
		fmt.Printf("\x1b[1;33m%4d\x1b[0m %s\n", i, strings.Join(fields, " "))
	}
}

func sql(sql string) []sqlite3.RowMap {
	var result []sqlite3.RowMap
	c, err := sqlite3.Open(DATABASE)
	if err != nil {
		panic(errorString("open", sql, err))
	}
	s, err := c.Query(sql)
	for {
		if err == io.EOF {
			break
		} else if err != nil {
			panic(errorString("fetch", sql, err))
		}
		row := make(sqlite3.RowMap)
		s.Scan(row)
		result = append(result, row)
		err = s.Next()
	}
	c.Commit()
	c.Close()
	logResult(sql, result)
	return result
}

// ----- http -----

type gtHandler struct {
	origHandler http.Handler
}

func (h *gtHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// you can set you own header for tracing etc
	w.Header().Add("X-Michurin", "Here!")
	// hacks for graphql-cli
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Add("Access-Control-Allow-Origin", origin)
	}
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type,X-Apollo-Tracing")
	if r.Method == http.MethodOptions {
		// just call for schema
		h.origHandler.ServeHTTP(w, r)
	} else {
		// fill request context
		h.origHandler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "dataloaders", NewLoaders())))
	}
}

func handlerWrapper(h http.Handler) *gtHandler {
	return &gtHandler{h}
}

// ----- util -----

func getLoaderFnByName(p graphql.ResolveParams, name string, key dataloader.Key) dataloader.Thunk {
	return p.Context.Value("dataloaders").(map[string]*dataloader.Loader)[name].Load(p.Context, key)
}

// ----- business objects -----

// Util

func callTrunkGetByName(trunk dataloader.Thunk, field string) func() (interface{}, error) {
	return func() (interface{}, error) {
		data, err := trunk()
		if err != nil {
			return nil, err
		}
		return data.(sqlite3.RowMap)[field], nil
	}
}

func callTrunkGetIdCast(trunk dataloader.Thunk, field string, caster func(int) interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		data, err := trunk()
		if err != nil {
			return nil, err
		}
		return caster(int(data.(sqlite3.RowMap)[field].(int64))), nil
	}
}

func callTrunkGetCompleteRides(trunk dataloader.Thunk) func() (interface{}, error) {
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
	}
}

func callTrunkGetCompleteRidesDeep(trunk dataloader.Thunk) func() (interface{}, error) {
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
				Driver:      NewDriverWithName(int(e["driver_id"].(int64)), e["name"].(string)),
				Customer:    NewCustomer(int(e["customer_id"].(int64))),
				Destination: e["destination"].(string),
			}
		}
		return r, nil
	}
}

// Driver
// features: two constructors to create prefilled structures, see deep_rides implementation in Customer.Resolve

type Driver struct {
	id   int
	name *string
}

func NewDriver(id int) *Driver {
	return &Driver{id: id}
}

func NewDriverWithName(id int, name string) *Driver {
	return &Driver{id: id, name: &name}
}

func (d *Driver) Resolve(p graphql.ResolveParams) (interface{}, error) {
	switch p.Info.FieldName {
	case "id":
		return d.id, nil // in fact, it is too lazy, we did not check is this id exists in db
	case "name":
		if d.name != nil {
			return d.name, nil
		}
		trunk := getLoaderFnByName(p, "driver", NewIntKey(d.id))
		return callTrunkGetByName(trunk, "name"), nil
	case "rides":
		trunk := getLoaderFnByName(p, "rides_by_driver_id", NewIntKey(d.id))
		return callTrunkGetCompleteRides(trunk), nil
	}
	return nil, errors.New("Driver resolver: Unknown field " + p.Info.FieldName)
}

// Customer
// features: extremely simple and minimal

type Customer struct {
	id int
}

func (c *Customer) Resolve(p graphql.ResolveParams) (interface{}, error) {
	switch p.Info.FieldName {
	case "id":
		return c.id, nil
	case "name":
		trunk := getLoaderFnByName(p, "customer", NewIntKey(c.id))
		return callTrunkGetByName(trunk, "name"), nil
	case "rides":
		trunk := getLoaderFnByName(p, "rides_by_customer_id", NewIntKey(c.id))
		return callTrunkGetCompleteRides(trunk), nil
	case "deep_rides":
		trunk := getLoaderFnByName(p, "deep_rides_by_customer_id", NewIntKey(c.id))
		return callTrunkGetCompleteRidesDeep(trunk), nil
	}
	return nil, errors.New("Customer resolver: Unknown field " + p.Info.FieldName)
}

func NewCustomer(id int) *Customer {
	return &Customer{id: id}
}

// Ride
// features: laziness and simplest implementation of prefilling just as separate structure without x.Resolve

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
		return callTrunkGetIdCast(trunk, "driver_id", func(id int) interface{} { return NewDriver(id) }), nil
	case "customer":
		trunk := r.getTrunk(p)
		return callTrunkGetIdCast(trunk, "customer_id", func(id int) interface{} { return NewCustomer(id) }), nil
	case "destination":
		trunk := r.getTrunk(p)
		return callTrunkGetByName(trunk, "destination"), nil
	}
	return nil, errors.New("Ride resolver: Unknown field " + p.Info.FieldName)
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
	fmt.Println("\x1b[1;34mLoaders created\x1b[0m")
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
		"rides_by_driver_id": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			return loadOneToMany("select * from Ride where driver_id in (%s)", "driver_id", keys)
		}),
		"rides_by_customer_id": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			return loadOneToMany("select * from Ride where customer_id in (%s)", "customer_id", keys)
		}),
		"deep_rides_by_customer_id": dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
			return loadOneToMany("select * from Ride join Driver using (driver_id) where customer_id in (%s)", "customer_id", keys)
		}),
	}
}

// ----- m.a.i.n -----

func main() {
	var driverType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Driver", // used by graphlql-relay
		Fields: graphql.Fields{
			"id":   &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"name": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	var customerType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Customer",
		Fields: graphql.Fields{
			"id":   &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"name": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	var rideType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Ride",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"driver":      &graphql.Field{Type: graphql.NewNonNull(driverType)},
			"customer":    &graphql.Field{Type: graphql.NewNonNull(customerType)},
			"destination": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	customerType.AddFieldConfig("rides", &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(rideType)))})
	customerType.AddFieldConfig("deep_rides", &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(rideType)))})
	driverType.AddFieldConfig("rides", &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(rideType)))})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"x_ride": &graphql.Field{
				Name: "ride",
				Type: rideType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					rideId := p.Args["id"].(int)
					return NewRide(rideId), nil // in fact, we have to check is rideId exists in db
				},
			},
			"x_rides": &graphql.Field{
				Name: "rides",
				Type: graphql.NewList(rideType),
				Args: graphql.FieldConfigArgument{
					"ids": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					rideIds := p.Args["ids"].([]interface{})
					rides := make([]*Ride, len(rideIds))
					for i, e := range rideIds {
						rides[i] = NewRide(e.(int))
					}
					return rides, nil
				},
			},
			"x_customer": &graphql.Field{
				Type: customerType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					customerId := p.Args["id"].(int)
					return NewCustomer(customerId), nil
				},
			},
		},
	})

	rideInputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "RideInput",
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
	})

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"add_ride": &graphql.Field{
				Name: "add_ride",
				Type: rideType,
				Args: graphql.FieldConfigArgument{
					"params": &graphql.ArgumentConfig{Type: graphql.NewNonNull(rideInputType)},
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
	})
	if err != nil {
		panic(err)
	}

	handler := handlerWrapper(handler.New(&handler.Config{
		Schema:     &schema,
		Pretty:     true,
		GraphiQL:   true,
		Playground: true,
	}))
	http.Handle("/gql", handler)

	fmt.Println(`
Examples:
  query { x_ride(id:2) {id destination customer {id name} driver {id name}} }
  query { x_customer(id: 200) {id name, rides {id, destination, driver {name}}} }
  query { x_ride(id: 3) {id destination customer {id name rides {id driver {name}}}} }
  query { x_ride(id: 3) {id destination customer {id name rides {id driver {name rides {id}}}}} }
  query { x_customer(id: 200) {rides{ driver{rides{ driver{rides{ driver{name} }} }} }} }
  query { x_rides(ids:[1 2]){id destination} }
  mutation { add_ride(params:{customer_id:100 driver_id:1 destination:"One"}){id, customer{name}} }
  query { x_customer(id: 200) {deep_rides{ driver{name} }} }
  query { x_customer(id: 200) {rides{ driver{name} }} }
Curl:
  curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d "$QUERY"
GraphiQL (in browser):
  http://localhost:8080/gql`)
	http.ListenAndServe(":8080", nil)
}
