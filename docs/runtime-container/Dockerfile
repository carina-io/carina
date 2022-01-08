FROM centos:7 as builder

RUN cd /tmp && curl -O https://ghproxy.com/https://github.com/g2p/bcache-tools/archive/refs/tags/v1.0.8.tar.gz && tar -zxvf v1.0.8.tar.gz
RUN yum -y install gcc automake autoconf libtool make gcc-c++ libblkid-devel
RUN cd /tmp/bcache-tools-1.0.8 && make && make install

FROM centos:7

ENV container docker

RUN yum --setopt=tsflags=nodocs -y update && yum clean all && \
(cd /lib/systemd/system/sysinit.target.wants/; for i in *; do [ $i == systemd-tmpfiles-setup.service ] || rm -f $i; done) && \
rm -f /lib/systemd/system/multi-user.target.wants/* &&\
rm -f /etc/systemd/system/*.wants/* &&\
rm -f /lib/systemd/system/local-fs.target.wants/* && \
rm -f /lib/systemd/system/sockets.target.wants/*udev* && \
rm -f /lib/systemd/system/sockets.target.wants/*initctl* && \
rm -f /lib/systemd/system/basic.target.wants/* &&\
rm -f /lib/systemd/system/anaconda.target.wants/* &&\
yum --setopt=tsflags=nodocs -y install nfs-utils && \
yum --setopt=tsflags=nodocs -y install attr  && \
yum --setopt=tsflags=nodocs -y install iputils  && \
yum --setopt=tsflags=nodocs -y install iproute  && \
yum --setopt=tsflags=nodocs -y install openssh-server  && \
yum --setopt=tsflags=nodocs -y install openssh-clients  && \
yum --setopt=tsflags=nodocs -y install rsync  && \
yum --setopt=tsflags=nodocs -y install tar  && \
yum --setopt=tsflags=nodocs -y install cronie  && \
yum --setopt=tsflags=nodocs -y install lvm2 && \
yum --setopt=tsflags=nodocs -y install parted && \
yum --setopt=tsflags=nodocs -y install file && \
yum --setopt=tsflags=nodocs -y install e4fsprogs && \
yum --setopt=tsflags=nodocs -y install xfsprogs  && yum clean all && \
sed -i '/Port 22/c\Port 2222' /etc/ssh/sshd_config && \
mkdir -p /var/log/core;

# do not run udev (if needed, bind-mount /run/udev instead?)
RUN true \
    && systemctl mask systemd-udev-trigger.service \
    && systemctl mask systemd-udevd.service \
    && systemctl mask systemd-udevd.socket \
    && systemctl mask systemd-udevd-kernel.socket \
    && true

# use lvmetad from the host, dont run it in the container
# don't wait for udev to manage the /dev entries, disable udev_sync, udev_rules in lvm.conf
VOLUME [ "/run/lvm" ]
RUN true \
    && systemctl mask lvm2-lvmetad.service \
    && systemctl mask lvm2-lvmetad.socket \
    && sed -i 's/^\sudev_rules\s*=\s*1/udev_rules = 0/' /etc/lvm/lvm.conf \
    && sed -i 's/^\sudev_sync\s*=\s*1/udev_sync= 0/' /etc/lvm/lvm.conf \
    && sed -i 's/use_lvmetad\s*=\s*1/use_lvmetad = 0/' /etc/lvm/lvm.conf \
    && sed -i 's/^\sobtain_device_list_from_udev\s*=\s*1/obtain_device_list_from_udev = 0/' /etc/lvm/lvm.conf \
    && true

# prevent dmeventd from running in the container, it may cause conflicts with
# the service running on the host
# monitoring of activated LVs can not be done inside the container
RUN true \
    && systemctl mask dm-event.service \
    && systemctl disable dm-event.socket \
    && systemctl mask dm-event.socket \
    && systemctl disable lvm2-monitor.service \
    && systemctl mask lvm2-monitor.service \
    && sed -i 's/^\smonitoring\s*=\s*1/monitoring = 0/' /etc/lvm/lvm.conf \
    && true

# mask services that aren't required in the container and/or might interfere
RUN true \
    && systemctl mask getty.target \
    && systemctl mask systemd-journal-flush.service \
    && systemctl mask rpcbind.socket \
    && true


VOLUME [ "/sys/fs/cgroup" ]
ADD update-params.sh /usr/local/bin/update-params.sh
ADD exec-on-host.sh /usr/sbin/exec-on-host

RUN chmod +x /usr/local/bin/update-params.sh && \
systemctl disable nfs-server.service

COPY --from=builder /tmp/bcache-tools-1.0.8/bcache-register /usr/bin/
COPY --from=builder /tmp/bcache-tools-1.0.8/bcache-super-show /usr/bin/
COPY --from=builder /tmp/bcache-tools-1.0.8/make-bcache /usr/bin/
COPY --from=builder /tmp/bcache-tools-1.0.8/probe-bcache /usr/bin/
RUN chmod +x /usr/bin/bcache-register /usr/bin/bcache-register /usr/bin/bcache-register /usr/bin/bcache-register

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/update-params.sh"]
CMD ["/usr/sbin/init"]
