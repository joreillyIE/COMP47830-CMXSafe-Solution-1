#!/bin/bash

chmod 600 /root/.ssh/id_ed25519

# Wait for SSH proxy to start
sleep 60

# Initialize user
MAC=$(cat /sys/class/net/eth0/address)
USER=$(echo "$MAC" | sed 's/://g')

# Print the found MAC address
echo "CMX Agent $USER Running..."

echo "Attempting to connect via SSH..."

while true; do
    ssh -L 1883:localhost:1883 -N -i /root/.ssh/id_ed25519 -o StrictHostKeyChecking=no -o ExitOnForwardFailure=yes $USER@dynamic-service-a23ff868ec8f.myservices.local -p 22 &
    SSH_PID=$!  # Capture SSH process ID
    sleep 180

    if ps -p $SSH_PID > /dev/null; then
        echo "SSH Transport Phase established successfully..."
    else
        wait $SSH_PID 2>/dev/null
        continue
    fi

    FAIL=0
    while ps -p $SSH_PID > /dev/null; do
        sleep 5
        if mosquitto_pub -h 127.0.0.1 -p 1883 -t "iot/data" -m "Hello from IoT Device 1"; then
            FAIL=0
        else
            ((FAIL++))
            [[ $FAIL -ge 5 ]] && break    # give up → kills loop, prints “Retrying…”
        fi
    done
    echo "Retrying..."
        # ---------- NEW ---------- #
    kill $SSH_PID 2>/dev/null || true      # stop old tunnel
    wait $SSH_PID 2>/dev/null  
done