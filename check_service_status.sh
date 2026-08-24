#!/bin/bash

read -p "Enter service name: " Service_name

echo "Checking Service Status..."

load_state=$(systemctl show -p LoadState --value "$Service_name")
active_state=$(systemctl show -p ActiveState --value "$Service_name")

if [ "$load_state" = "not-found" ] || [ -z "$load_state" ]; then
    echo "Service '$Service_name' is not found"
elif [ "$active_state" = "active" ]; then
    echo "Service '$Service_name' is running"
else
    echo "Service '$Service_name' is not running (Status: $active_state)"
fi