FROM ubuntu:24.04

RUN apt update && apt install -y --no-install-recommends openssh-client mosquitto-clients dos2unix iproute2 && rm -rf /var/lib/apt/lists/*

COPY start.sh /start.sh
RUN chmod +x /start.sh
RUN dos2unix /start.sh && chmod +x /start.sh

CMD ["/start.sh"]
