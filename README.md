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

```graphql
schema {
  query: RootQuery
  mutation: Mutation
}

type customer {
  deep_rides: [ride]
  id: Int
  name: String
  rides: [ride]
}

type driver {
  id: Int
  name: String
  rides: [ride]
}

type Mutation {
  add_ride(params: rideInput!): ride
}

type ride {
  customer: customer
  destination: String
  driver: driver
  id: Int
}

input rideInput {
  driver_id: Int!
  destination: String!
  customer_id: Int!
}

type RootQuery {
  x_customer(id: Int!): customer
  x_ride(id: Int!): ride
  x_rides(ids: [Int!]!): [ride]
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

#### Related tools

- [graphql-cli](https://github.com/graphql-cli/graphql-cli)
