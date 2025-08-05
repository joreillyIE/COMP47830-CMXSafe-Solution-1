FROM ubuntu:latest

RUN apt update && apt install -y iputils-ping iputils-arping dnsutils openssh-client mosquitto-clients dos2unix net-tools tcpdump iproute2 && rm -rf /var/lib/apt/lists/*

COPY start.sh /start.sh
RUN chmod +x /start.sh

# Convert scripts to UNIX format
RUN dos2unix /start.sh && chmod +x /start.sh

CMD ["/start.sh"]
