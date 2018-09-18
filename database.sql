create table Ride (ride_id int, driver_id int, customer_id int, destination string);
create table Driver (driver_id int, name string);
create table Customer (customer_id int, name string);

insert into Customer values (100, "C100"), (200, "C200");
insert into Driver values (1, "D1"), (2, "D2");
insert into Ride values (1, 1, 100, "R1"), (2, 1, 200, "R2"), (3, 2, 200, "R3");
-- select * from Ride join Driver using (driver_id) join Customer using (customer_id);
-- 1|1|100|R1|D1|C100
-- 2|1|200|R2|D1|C200
-- 3|2|200|R3|D2|C200
