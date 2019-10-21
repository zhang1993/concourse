#!/bin/bash

set -o errexit
set -o nounset

git clone --branch k8s-topgun-maybe-forward https://github.com/concourse/concourse
git clone https://github.com/helm/charts

mkdir -p ~/.kube
echo "$KUBE_CONFIG" > ~/.kube/config
