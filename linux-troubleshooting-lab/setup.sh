#!/bin/bash

mkdir -p /var/www/demo

cp /tmp/files/index.html /var/www/demo/index.html

cp /tmp/files/nginx.conf /etc/nginx/nginx.conf

mkdir -p /etc/nginx/conf.d

cp /tmp/files/app.conf /etc/nginx/conf.d/app.conf

cp /tmp/files/broken-service.service /etc/systemd/system/

systemctl enable nginx

systemctl daemon-reload

mkdir -p /var/log/lab

dd if=/dev/zero of=/var/log/lab/big.log bs=10M count=50

touch /var/log/lab/app.log

chmod -R 755 /var/www

echo "Setup Complete"