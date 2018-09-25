### Playing with [GraphQL](http://graphql.org)

- [Golang](http://golang.org)
- [GraphQL](http://github.com/graphql-go/graphql)
- [DataLoader](http://github.com/graph-gophers/dataloader)
- [HTTP](http://github.com/graphql-go/handler)

#### Install

```sh
go get github.com/michurin/playground-graphql-go
```

#### Setup

```sh
$GOPATH/src/github.com/michurin/playground-graphql-go/database_init.sh
```

By the way, you can easily view db using `database_show.sh`.

#### Run

```sh
$GOPATH/bin/playground-graphql-go
```
or
```sh
go run $GOPATH/src/github.com/michurin/playground-graphql-go/main.go
```

#### Enjoy

```sh
Q='query {x_customer(id: 200) {name rides {destination driver {rides {destination customer{name}}}}}}'
curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d "$Q"
```

```javascript
{
    "data": {
        "x_customer": {
            "name": "Customer_200",
            "rides": [
                {
                    "destination": "Address_for_ride_2",
                    "driver": {
                        "rides": [
                            {
                                "customer": {
                                    "name": "Customer_100"
                                },
                                "destination": "Adderss_for_ride_1"
                            },
                            {
                                "customer": {
                                    "name": "Customer_200"
                                },
                                "destination": "Address_for_ride_2"
                            }
                        ]
                    }
                },
                {
                    "destination": "Address_for_ride_3",
                    "driver": {
                        "rides": [
                            {
                                "customer": {
                                    "name": "Customer_200"
                                },
                                "destination": "Address_for_ride_3"
                            }
                        ]
                    }
                }
            ]
        }
    }
}
```

#### GraphQL schema

```
type Query {
    x_ride(id:Int!): Ride
    x_rides(ids:[Int]!): [Ride]
    x_customer(id:Int!): Customer
}
type Ride {
    id: Int!
    destination: String!
    driver: Driver!
    customer: Customer!
}
type Driver {
    id: Int!
    name: String!
    rides: [Ride]!
}
type Customer {
    id: Int!
    name: String!
    rides: [Ride]!
    deep_rides: [Ride]!
}
```

#### Database schema

```
                Ride
Driver          +-------------+
+-----------+   | ride_id     |   Customer
| driver_id |--<| driver_id   |   +-------------+
| name      |   | customer_id |>--| customer_id |
+-----------+   | destination |   | name        |
                +-------------+   +-------------+
```

More details in `database_init.sh` script.
