#!/bin/bash

chmod 600 /root/.ssh/id_ed25519

# Wait for SSH proxy to start
sleep 60

# Initialize user
MAC=$(cat /sys/class/net/eth0/address)
USER=$(echo "$MAC" | sed 's/://g')

# Print the found MAC address
echo "CMX Agent $USER Running..."

echo "Starting MQTT broker..."
mosquitto -c /etc/mosquitto/mosquitto.conf -d

echo "Attempting to connect via SSH..."
while true; do
    
    ssh -R localhost:1883:localhost:1883 -i /root/.ssh/id_ed25519 -o StrictHostKeyChecking=no -o ConnectTimeout=5 $USER@cmxsafe-gw.myservices.local -p 22 &
    
    SSH_PID=$!  # Capture SSH process ID
    sleep 180  # Give some time to establish the connection

    # Check if SSH connection is still running
    if ps -p $SSH_PID > /dev/null; then
        echo "SSH tunnel established successfully!"
        break
    else
        sleep 5
    fi
done

# Wait for SSH proxy to start
sleep 5

echo "Starting MQTT subscriber..."
mosquitto_sub -h 127.0.0.1 -t "iot/data"