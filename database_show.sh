#!/bin/sh

sqlite3 database.db '.headers on' '.mode column' 'select * from Ride join Driver using(driver_id) join Customer using(customer_id);'
