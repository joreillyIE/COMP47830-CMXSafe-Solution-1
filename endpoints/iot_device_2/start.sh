#!/bin/bash

sleep 5

MAC_GW="2001:0242:6392:f935"
MAC_SRV="a2:3f:f8:68:ec:8f"
MAC_DEV=$(cat /sys/class/net/eth0/address)

# Initialize user
USER=$(echo "$MAC_DEV" | sed 's/://g')
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
DEV_ADDR=$(get_vip "$MAC_GW" "$MAC_DEV")
GW_ADDR_1="cmxsafe-gw.myservices.local"
GW_ADDR_2="a23ff868ec8f.default.svc.cluster.local"

ip -6 addr add "$SRV_ADDR"/128 dev lo

# Target Ports
SSH_port=22
MQTT_port=1883

# Fork into a subshell as user
(

    exec su - "$USER" <<EOF

    echo "CMX Agent $USER Running..."

    # Wait for SSH proxy to start
    sleep 120

    echo "Attempting to connect via SSH to $GW_ADDR_1 and then $GW_ADDR_2..."

    while true; do
        sleep 5
        ssh -L [$SRV_ADDR]:$MQTT_port:[$SRV_ADDR]:$MQTT_port -N -i /home/$USER/.ssh/id_ed25519 -o ConnectTimeout=20 -J $USER@$GW_ADDR_1:$SSH_port $USER@$GW_ADDR_2 -p $SSH_port
    done

EOF
) &

sleep 180
while true; do
    sleep 5
    mosquitto_pub -A ::1 -h $SRV_ADDR -p $MQTT_port -u $DEV_ADDR -t "iot/data" -m "Hello from IoT Device 2" --repeat 9999 --repeat-delay 5
done