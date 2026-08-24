FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y \
    nginx \
    systemd \
    systemd-sysv \
    iproute2 \
    procps \
    net-tools \
    curl \
    vim \
    sudo && \
    apt-get clean

COPY setup.sh /setup.sh
COPY files/ /tmp/files/

RUN chmod +x /setup.sh && /setup.sh

CMD ["/sbin/init"]