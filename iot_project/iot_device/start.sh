#!/bin/bash

chmod 600 /root/.ssh/id_rsa

# Wait for SSH proxy to start
sleep 60

# Initialize user
MAC=$(cat /sys/class/net/eth0/address)
USER=$(echo "$MAC" | sed 's/://g')

# Print the found MAC address
echo "CMX Agent $USER Running..."

echo "Attempting to connect via SSH..."
while true; do
    
    ssh -L 1883:localhost:1883 -N -i /root/.ssh/id_rsa -o StrictHostKeyChecking=no $USER@dynamic-service-a23ff868ec8f.myservices.local -p 22 &
    
    SSH_PID=$!  # Capture SSH process ID
    sleep 10  # Give some time to establish the connection

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

echo "Starting MQTT publisher..."

while true; do
  mosquitto_pub -h 127.0.0.1 -p 1883 -t "iot/data" -m "Hello from IoT Device"
  sleep 5
done
