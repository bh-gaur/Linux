#!/bin/bash

# Script to print days of the week

days=("Monday" "Tuesday" "Wednesday" "Thursday" "Friday" "Saturday" "Sunday")

echo "Days of the week:"
for day in "${days[@]}"
do
    echo "$day"
done

