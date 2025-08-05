FROM ubuntu:latest

RUN apt update && apt install -y openssh-client mosquitto mosquitto-clients dos2unix && rm -rf /var/lib/apt/lists/*

RUN echo "listener 1883" >> /etc/mosquitto/mosquitto.conf
RUN echo "allow_anonymous true" >> /etc/mosquitto/mosquitto.conf

COPY start.sh /start.sh
RUN chmod +x /start.sh

# Convert scripts to UNIX format
RUN dos2unix /start.sh && chmod +x /start.sh

CMD ["/start.sh"]