#!/bin/bash
# Script to update the parameters passed to container image

: ${GB_GLFS_LRU_COUNT:=15}
: ${GB_CLI_TIMEOUT:=900}
: ${HOST_DEV_DIR:=/mnt/host-dev}
: ${CGROUP_PIDS_MAX:=max}
: ${TCMU_LOCKDIR:=/var/run/lock}


if [ -c "${HOST_DEV_DIR}/zero" ] && [ -c "${HOST_DEV_DIR}/null" ]; then
    # looks like an alternate "host dev" has been provided
    # to the container. Use that as our /dev ongoing
    mount --rbind "${HOST_DEV_DIR}" /dev
fi

# Hand off to CMD
exec "$@"
