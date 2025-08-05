#!/usr/bin/env bash

logger "createservice.sh was invoked at $(date) with argument $1" >> /tmp/createservice.log
logger "DEBUG: createservice.sh was invoked"
sleep 100



#printenv >> /tmp/createservice.log

#set -e

# Optional: get the Pod name for dynamic naming (Pod name often == hostname)
# This works if your container's hostname is set to the Pod name
# or use the Downward API for absolute reliability

#ip_dash="${IP//./-}"

#until kubectl apply -f - <<EOF
#apiVersion: v1
#kind: Service
#metadata:
#  name: ${USER}-1883
#spec:
#  type: ExternalName
#  externalName: ${ip_dash}.cmxsafe-gw.default.svc.cluster.local
#EOF
#do
#    echo "[$(date)] Internal Service creation failed, retrying in 5 seconds..." >> /tmp/createservice.log
#    sleep 5
#done


# Now attempt to create the service until kubectl apply is successful.
#until kubectl apply -f - <<EOF
#apiVersion: v1
#kind: Service
#metadata:
#  name: dynamic-service-${USER}
#  annotations:
#    external-dns.alpha.kubernetes.io/hostname: dynamic-service-${USER}.myservices.local
#  labels:
#    createdBy: ${MY_POD_NAME}
#spec:
#  type: LoadBalancer
#  selector:
#    pod-name: ${MY_POD_NAME}
#  ports:
#    - name: ssh-primary
#      port: 22
#      targetPort: 22
#      protocol: TCP 
#    - name: ssh-secondary
#      port: 2222
#      targetPort: 2222
#      protocol: TCP  
#EOF
#do
#    echo "[$(date)] External Service creation failed, retrying in 5 seconds..." >> /tmp/createservice.log
#    sleep 5
#done

#echo "[$(date)] Service successfully created." >> /tmp/createservice.log