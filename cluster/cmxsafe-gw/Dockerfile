FROM ubuntu:24.04 AS builder
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y build-essential zlib1g-dev libssl-dev libedit-dev pkg-config autoconf automake libtool dos2unix

COPY openssh/openssh-portable-V_9_2_P1 /src/openssh
WORKDIR /src/openssh

RUN autoreconf && ./configure --prefix=/usr --sysconfdir=/etc/ssh --with-privsep-path=/var/lib/sshd --without-zlib-version-check && make -j$(nproc) && make install-nokeys DESTDIR=/tmp/install

COPY createservice.sh /tmp/createservice.sh
RUN chmod +x /tmp/createservice.sh && dos2unix /tmp/createservice.sh

# -------------------------

FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends zlib1g libssl3 && rm -rf /var/lib/apt/lists/*

# Copy OpenSSH from builder
COPY --from=builder /tmp/install/ /

# Create runtime dirs
RUN mkdir -p /var/run/sshd /var/lib/sshd

RUN useradd -r -M -s /usr/sbin/nologin sshd

COPY --from=builder /tmp/createservice.sh /etc/createservice.sh
RUN chmod +x /etc/createservice.sh

EXPOSE 22
CMD ["/bin/sh"]