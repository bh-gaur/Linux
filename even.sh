#!/bin/bash

# write a bash script to print even numbers from 1 to 10

# numbers=(1,2,3,4,5,6,7,8,9,10)

print_even_numbers(){
    echo Even Numbers
    # for num in "${numbers[@]}"
    for i in {1..10}
    do
    if (( i % 2 == 0 ))
    then
    echo $i
    fi
    done
}

print_odd_numbers(){
    echo Odd Numbers
    # for num in "${numbers[@]}"
    for i in {1..10}
    do
    if (( i % 2 == 1 ))
    then
    echo $i
    fi
    done
}

print_even_numbers

banner and

print_odd_numbers