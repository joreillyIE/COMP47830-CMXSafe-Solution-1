# COMP47830-CMXSafe-Solution-1

## Description
- Setup script deploys a kind cluster one worker node and three containers which act as a domain name server, IoT server, and IoT device.
- On start up, the IoT server ssh client tries to connect to cmxsafe-gw.myservices.local
- When the k8s load balancer service named cmxsafe-gw is launched, an external IPv4 address is provided by MetalLB.
- External DNS requests domain name cmxsafe-gw.myservices.local for external IPv4 address.
- IoT server is then able to resolve cmxsafe-gw.myservices.local and opens ssh connection with a replica pod.
- Load balancer service is configured to allow session affinity so connection stays uninterupted for up to 24 hours.
- When IoT server connects to a pod, a script is automatically executed to create another service named dynamic-service-<IOT_SERVER_MAC_ADDR>.
- This service points onto to this particular pod, receives an external IPv4 address and is provisioned domain name dynamic-service-<IOT_SERVER_MAC_ADDR>.myservices.local by External DNS.
- IoT device is then able to resolve dynamic-service-<IOT_SERVER_MAC_ADDR>.myservices.local and opens ssh connection with the same replica pod.

## Prerequisites
- Docker Desktop
- Kind
- Kubectl

## How to run
- Download files.
- Set environment variable CMXSafeProject to location of downloaded files.
- In terminal, go to location of files and run
  .\setup.ps1

