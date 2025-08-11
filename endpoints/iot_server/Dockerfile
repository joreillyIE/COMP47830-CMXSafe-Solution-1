FROM ubuntu:24.04 AS builder
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y build-essential zlib1g-dev libssl-dev libedit-dev pkg-config autoconf automake libtool dos2unix

COPY openssh/openssh-portable-V_9_2_P1 /src/openssh
WORKDIR /src/openssh

RUN autoreconf && ./configure --prefix=/usr --sysconfdir=/etc/ssh --with-privsep-path=/var/lib/sshd --without-zlib-version-check && make -j$(nproc) && make install-nokeys DESTDIR=/tmp/install

COPY start.sh /tmp/start.sh
RUN chmod +x /tmp/start.sh && dos2unix /tmp/start.sh

# -------------------------

FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive

# RUN apt update && apt install -y openssh-client mosquitto mosquitto-clients dos2unix && rm -rf /var/lib/apt/lists/*
RUN apt-get update && apt-get install -y --no-install-recommends zlib1g libssl3 iproute2 mosquitto mosquitto-clients && rm -rf /var/lib/apt/lists/*

# Copy OpenSSH from builder
COPY --from=builder /tmp/install/ /

# Create runtime dirs
RUN mkdir -p /var/run/sshd /var/lib/sshd && chmod 0755 /var/run/sshd

RUN echo "listener 1883 ::1" >> /etc/mosquitto/mosquitto.conf
RUN echo "allow_anonymous true" >> /etc/mosquitto/mosquitto.conf

COPY --from=builder /tmp/start.sh /start.sh
RUN chmod +x /start.sh

CMD ["/start.sh"]