#!/bin/sh

cat <<__END__ | sqlite3 database.db
drop table if exists Ride;
drop table if exists Driver;
drop table if exists Customer;
create table Ride (ride_id int, driver_id int, customer_id int, destination string);
create table Driver (driver_id int, name string);
create table Customer (customer_id int, name string);

insert into Customer values
  (100, "Customer_100"),
  (200, "Customer_200");
insert into Driver values
  (1, "Driver_1"),
  (2, "Driver_2");
insert into Ride values
  (1, 1, 100, "Adderss_for_ride_1"),
  (2, 1, 200, "Address_for_ride_2"),
  (3, 2, 200, "Address_for_ride_3");
select * from Ride join Driver using (driver_id) join Customer using (customer_id);
-- 1|1|100|R1|D1|C100
-- 2|1|200|R2|D1|C200
-- 3|2|200|R3|D2|C200
__END__
