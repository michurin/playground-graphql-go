#!/bin/sh

cat <<__END__ | sqlite3 database.db
PRAGMA foreign_keys=ON;
BEGIN TRANSACTION;
DROP TABLE IF EXISTS Ride;
DROP TABLE IF EXISTS Driver;
DROP TABLE IF EXISTS Customer;
CREATE TABLE Driver (
  driver_id integer primary key autoincrement,
  name string);
insert INTO Driver VALUES(1,'Driver_1');
insert INTO Driver VALUES(2,'Driver_2');
CREATE TABLE Customer (
  customer_id integer primary key autoincrement,
  name string);
insert INTO Customer VALUES(100,'Customer_100');
insert INTO Customer VALUES(200,'Customer_200');
CREATE TABLE Ride (
  ride_id integer primary key autoincrement,
  driver_id integer references Driver,
  customer_id integer references Customer,
  destination string);
insert INTO Ride VALUES(1,1,100,'Adderss_for_ride_1');
insert INTO Ride VALUES(2,1,200,'Address_for_ride_2');
insert INTO Ride VALUES(3,2,200,'Address_for_ride_3');
COMMIT;
__END__
