#!/usr/bin/env bash



openssl req -x509 -newkey rsa:4096 -days 365 -nodes -keyout ca-key.pem -out ca-cert.pem -subj "/C=UK/ST=London/L=London/O=Local/OU=LiquidMetal/CN=*.liquidmetal.local/emailAddress=someone@somewhere.dev"
openssl req -newkey rsa:4096 -nodes -keyout server-key.pem -out server-req.pem -subj "/C=UK/ST=London/L=London/O=Local/OU=LiquidMetal/CN=*.flintlock.local/emailAddress=someone@somewhere.dev"
openssl x509 -req -in server-req.pem -days 60 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem
