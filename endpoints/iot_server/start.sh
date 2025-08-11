#!/bin/bash

sleep 5

MAC_GW="2001:0242:6392:f935"
MAC_DEV_1="02:5f:9a:3d:b7:12"
MAC_DEV_2="02:42:ac:11:00:02"
MAC_SRV=$(cat /sys/class/net/eth0/address)

# Initialize user
USER=$(echo "$MAC_SRV" | sed 's/://g')
useradd -m $USER
if [ -d "/home/$USER/.ssh" ]; then
    chown -R "$USER":"$USER" "/home/$USER"
    chmod 700 "/home/$USER/.ssh"
    chmod 600 "/home/$USER/.ssh/id_ed25519"
fi

get_vip() {
  local prefix="$1" mac="$2"

  mac="$(echo -n "$mac" | tr '[:upper:]' '[:lower:]' | sed 's/[:.\-]//g')"
  [[ "$mac" =~ ^[0-9a-f]{12}$ ]] || { echo "invalid MAC" >&2; return 1; }

  local left="${mac:0:6}" right="${mac:6:6}"
  local eui64="${left}fffe${right}"
  local iid
  iid="$(echo "$eui64" | sed 's/.\{4\}/&:/g; s/:$//')"

  # ensure exactly one ':' between prefix and IID
  [[ $prefix == *: ]] || prefix="${prefix}:"
  printf '%s%s\n' "$prefix" "$iid"
}

# Initialize Addresses
SRV_ADDR=$(get_vip "$MAC_GW" "$MAC_SRV")
DEV_1_ADDR=$(get_vip "$MAC_GW" "$MAC_DEV_1")
DEV_2_ADDR=$(get_vip "$MAC_GW" "$MAC_DEV_2")
GW_ADDR="cmxsafe-gw.myservices.local"

ip -6 addr add "$DEV_1_ADDR"/128 dev lo
ip -6 addr add "$DEV_2_ADDR"/128 dev lo

# Target Ports
SSH_port=22
MQTT_port=1883

echo "Starting MQTT broker..."
mosquitto -c /etc/mosquitto/mosquitto.conf -d

# Fork into a subshell as user
(

    exec su - "$USER" <<EOF

    echo "CMX Agent $USER Running..."

    # Wait for SSH proxy to start
    sleep 60

    echo "Attempting to connect via SSH to $GW_ADDR..."

    while true; do
        sleep 5
        ssh -R [$SRV_ADDR]:$MQTT_port:[::1]:$MQTT_port -i /home/$USER/.ssh/id_ed25519 $USER@$GW_ADDR -p $SSH_port
    done

EOF
) &

# Wait for SSH proxy to start
sleep 5

echo "Starting MQTT subscriber..."
mosquitto_sub -A ::1 -h ::1 -p 1883 -u $SRV_ADDR -t "iot/data"