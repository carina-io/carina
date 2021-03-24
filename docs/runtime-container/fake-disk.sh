#!/bin/bash

# Set the USE_FAKE_DISK environment variable in the container deployment
#USE_FAKE_DISK=1
# You should also have a bind-mount for /srv in case data is expected to stay
FAKE_DISK_FILE=${FAKE_DISK_FILE:-/srv/fake-disk.img}
FAKE_DISK_SIZE=${FAKE_DISK_SIZE:-10G}
FAKE_DISK_DEV=${FAKE_DISK_DEV:-/dev/fake}

# Create the FAKE_DISK_FILE with fallocate, but only do so if it does not exist
# yet.
create_fake_disk_file () {
  [ -e "${FAKE_DISK_FILE}" ] && return 0
  truncate --size "${FAKE_DISK_SIZE}" "${FAKE_DISK_FILE}"
}

# Setup a loop device for the FAKE_DISK_FILE, and create a symlink to /dev/fake
# so that it has a stable name and can be used by other components (/dev/loop*
# is numbered based on other existing loop devices).
setup_fake_disk () {
  local fakedev

  fakedev=$(losetup --find --show "${FAKE_DISK_FILE}")
  [ -e "${fakedev}" ] && ln -fs "${fakedev}" "${FAKE_DISK_DEV}"
}

if [ -n "${USE_FAKE_DISK}" ]
then
  if ! create_fake_disk_file
  then
    echo "failed to create a fake disk at ${FAKE_DISK_FILE}"
    exit 1
  fi

  if ! setup_fake_disk
  then
    echo "failed to setup loopback device for ${FAKE_DISK_FILE}"
    exit 1
  fi
fi
