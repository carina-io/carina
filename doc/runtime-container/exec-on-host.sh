#!/bin/sh
#
# Privilege escalation detection
# - run command on the host, instead of in the container
# - in case of misconfiguration, run the command in the container anyway
#

HOST_ROOTFS=${HOST_ROOTFS:-'/rootfs'}
HOST_ESCAPE="nsenter --root=${HOST_ROOTFS} --mount=${HOST_ROOTFS}/proc/1/ns/mnt --ipc=${HOST_ROOTFS}/proc/1/ns/ipc --net=${HOST_ROOTFS}/proc/1/ns/net --uts=${HOST_ROOTFS}/proc/1/ns/uts"

error() {
	echo "${@}" > /dev/stderr
	echo "Running command inside container: ${COMMAND}" > /dev/stderr
	${COMMAND_FULL}
	exit $?
}

COMMAND="${1}"
COMMAND_FULL="${*}"

# detect /-filesystem of the host
if [ ! -d "${HOST_ROOTFS}" ]
then
	error "The /-filesystem of the host is not at ${HOST_ROOTFS}"
fi

# check if the HOST_ESCAPE works by running /bin/true on the host
if ! ${HOST_ESCAPE} /bin/true
then
	error "Could not run a command on the host, falling back to run in container."
fi

echo "Running command on the host: ${COMMAND}" > /dev/stderr
# shellcheck disable=SC2086, arguments should not be escaped
exec ${HOST_ESCAPE} ${COMMAND_FULL}
