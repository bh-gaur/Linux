#!/bin/bash

# # Ask user for input
read -p "Enter a number to print its table: " num

# echo "Multiplication Table for $num:"
# for i in {1..10}
# do
#     echo "$num x $i = $((num * i))"
# done


# echo "Enter a number to print its table: "
# read num

for i in {1..10}
do
    echo "$num x $i = $((num * i))"
done