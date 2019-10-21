#!/bin/bash

# notes.: - this is supposed to be run within `topgun` cluster.
#         - `config.yml` is already configured to target `topgun`
#

set -o errexit
set -o nounset

kubectl run \
    -i --tty --attach --pod-running-timeout=5m \
    topgun \
    --image concourse/unit \
    --env CHARTS_DIR=/go/charts \
    --env CONCOURSE_IMAGE_NAME=concourse/concourse \
    --env KUBE_CONFIG="$(cat ./config.yml)"
