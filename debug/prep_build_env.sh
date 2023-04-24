#!/usr/bin/env bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

cd $SCRIPT_DIR

rm -f go.mod go.sum 
go mod init caddy 
go mod tidy 
go mod edit -replace github.com/mietzen/caddy-dynamicdns-cmd-source=/Users/nils/Developer/caddy-dynamicdns-cmd-source 
