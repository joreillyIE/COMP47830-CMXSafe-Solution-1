FROM ubuntu:latest

RUN apt update && apt install -y iputils-ping iputils-arping dnsutils openssh-client mosquitto mosquitto-clients net-tools dos2unix tcpdump iproute2 proxychains4 && rm -rf /var/lib/apt/lists/*

RUN echo "listener 1883" >> /etc/mosquitto/mosquitto.conf
RUN echo "allow_anonymous true" >> /etc/mosquitto/mosquitto.conf

COPY start.sh /start.sh
RUN chmod +x /start.sh

# Convert scripts to UNIX format
RUN dos2unix /start.sh && chmod +x /start.sh

CMD ["/start.sh"]