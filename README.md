### Playing with [GraphQL](http://graphql.org)

- [Golang](http://golang.org)
- [GraphQL](http://github.com/graphql-go/graphql)
- [DataLoader](http://github.com/graph-gophers/dataloader)
- [HTTP](github.com/graphql-go/handler)

```sh
go run main.go
```
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
DB requsests are batched:
```sql
select * from Customer where customer_id=200;
select * from Ride where customer_id in (200);
select * from Driver where driver_id in (1, 2);
select * from Ride where driver_id in (1, 2);
select * from Customer where customer_id in (100, 200);
```
DB content:
```sh
sqlite3 database.db .dump
```
```sql
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE Ride (ride_id int, driver_id int, customer_id int, destination string);
INSERT INTO Ride VALUES(1,1,100,'Adderss_for_ride_1');
INSERT INTO Ride VALUES(2,1,200,'Address_for_ride_2');
INSERT INTO Ride VALUES(3,2,200,'Address_for_ride_3');
CREATE TABLE Driver (driver_id int, name string);
INSERT INTO Driver VALUES(1,'Driver_1');
INSERT INTO Driver VALUES(2,'Driver_2');
CREATE TABLE Customer (customer_id int, name string);
INSERT INTO Customer VALUES(100,'Customer_100');
INSERT INTO Customer VALUES(200,'Customer_200');
COMMIT;
```
