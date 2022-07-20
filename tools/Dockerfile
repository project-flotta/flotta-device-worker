# FROM quay.io/podman/stable:v4.2.0
# Hack until 4.2 is released
FROM quay.io/podman/testing:ae7690c42379

WORKDIR /project
RUN dnf install -y 'dnf-command(copr)'
RUN dnf copr enable project-flotta/flotta-testing -y

# Yum dependencies
RUN dnf update -y \
  && dnf install -y openssl procps-ng dmidecode nc iproute \
  yggdrasil flotta-agent-race node_exporter systemd-container \
  libselinux-utils util-linux sssd-client

# Modify podman configuration
RUN sed -i s/netns=\"host\"/netns=\"private\"/g /etc/containers/containers.conf && \
    sed -i s/utsns=\"host\"/utsns=\"private\"/g /etc/containers/containers.conf && \
    sed -i s/ipcns=\"host\"/ipcns=\"private\"/g /etc/containers/containers.conf

# Certificate reqs:
RUN mkdir /etc/pki/consumer && \
    openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes -out cert.pem -keyout key.pem -subj "/C=EU/ST=No/L=State/O=D/CN=www.example.com" && \
    mv cert.pem key.pem /etc/pki/consumer

# Default yggdrasil configuration should be replaced by volume with proper config:
RUN echo "" > /etc/yggdrasil/config.toml && \
    echo 'key-file = "/etc/pki/consumer/key.pem"' >> /etc/yggdrasil/config.toml && \
    echo 'cert-file = "/etc/pki/consumer/cert.pem"' >> /etc/yggdrasil/config.toml && \
    echo 'server = "project-flotta.io:8043"' >> /etc/yggdrasil/config.toml && \
    echo 'protocol = "http"' >> /etc/yggdrasil/config.toml && \
    echo 'path-prefix="api/flotta-management/v1"' >> /etc/yggdrasil/config.toml && \
    echo 'log-level="trace"' >> /etc/yggdrasil/config.toml

ENTRYPOINT ["/sbin/init"]
