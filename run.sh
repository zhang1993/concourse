#!/bin/bash

set -o errexit
set -o nounset

main () {
	gofmt -r "$1 -> berrors.$1" -w ./baggageclaim
}

main "$@"
