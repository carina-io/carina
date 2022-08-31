#!/bin/bash
set -e

NC='\e[0m'
BGREEN='\e[32m'

SLOW_E2E_THRESHOLD=${SLOW_E2E_THRESHOLD:-5}
FOCUS=${FOCUS:-.*}
E2E_NODES=${E2E_NODES:-5}
E2E_SKIP=${E2E_SKIP}
export ACK_GINKGO_RC=true
ginkgo_args=(
  "-randomizeAllSpecs"
  "-flakeAttempts=2"
  "-failFast"
  "-progress"
  "-slowSpecThreshold=${SLOW_E2E_THRESHOLD}"
  "-succinct"
  "-timeout=75m"
  "-v"
)



if [[ ${E2E_SKIP} != "" ]]; then
  echo -e "${BGREEN}Running e2e test suite (FOCUS=${FOCUS})...${NC}"
  ginkgo "${ginkgo_args[@]}"               \
    -focus="${FOCUS}"                  \
    -nodes="${E2E_NODES}"           \
    -skip="${E2E_SKIP}"
else
  ginkgo "${ginkgo_args[@]}"               \
    -focus="${FOCUS}"                  \
    -nodes="${E2E_NODES}"
fi



