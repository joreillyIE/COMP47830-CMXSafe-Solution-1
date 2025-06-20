# CMXsafe Solution for Connection Oriented Communication via Intermediate Proxies (Draft ReadMe v1.2)

## About Project

This project implements a containerized Kubernetes-based architecture that solves a key limitation in the CMXsafe proxy layer architecture for IoT security: ensuring connection-oriented communication continuity across stateless CMX-Gateway (CMX-GW) replicas. The solution addresses the stateful behavior of socket connections in Kubernetes environments and enables dynamic, secure, and scalable communication routing.

### What is this project?

This project is a practical implementation of the CMXsafe proxy layer for IoT communication security. It adapts the architecture for Kubernetes environments, enabling reliable routing of communication between IoT devices and platforms through multiple CMX-GW replicas, using clustering, session affinity, and domain name based routing.

### Why is this project important?

CMXsafe is designed to improve IoT communication security by decoupling authentication and encryption from the device firmware, reducing vulnerabilities and easing updates. The key idea behind this is that IoT communication occurs over socket communication paths setup with proxies known as CMX Gateways. CMX Gateways are stateless in nature, behave autonomously and are deployed as immutable containers. However, communication via CMX-GWs is connection oriented and therefore it depends on state. This means that CMX-GWs could fail to maintain consistent socket communication paths. The stateful problem of proxy communication a foundational challenge in modern distributed systems.

### What goals does this project accommplish?

Our research explored how to best implement the CMXsafe system using microservices. Any possible solutions had to address the problem of working connection-based communication through stateless proxies, and the solution incurring least overhead was implemented. We designed an approach that meets these goals by using Kubernetes Cluster technology, DNS based routing and session affinity.

## Background and Features

CMXsafe is a secure-by-design, application-agnostic proxy layer for securing IoT communications. It acts at the OSI transport layer (Layer 4) and intermediates traffic between IoT devices and platforms without requiring modifications to firmware or application code. Our solution implements CMXsafe using Kubernetes- the de facto standard for container orchestration. Kubernetes provides automated scheduling, self-healing, and horizontal autoscaling, ideally suited to the proxy layer’s stateless, containerized design.

### CMXsafe Components
- CMX Orchestrator: The management and policy coordination component of CMXSafe.
- CMX Gateways (CMX-GWs): Stateless, containerizable socket proxy servers that act as intermediaries for all device-server communications.
- Agents: Lightweight client-side proxies on IoT devices and servers that initiate Secure Proxy Sessions (SPSs) using transport layer protocols.
- Secure Proxy Sessions (SPS): Encrypted, authenticated sessions that connect agents and gateways.
- Identity (ID) and Mirror (Mir) Sockets: Ensure the preservation of device identity within proxied traffic using virtual IPv6 mappings.
- Security Contexts (SCs): Enforce inter-process communication policies at the OS level by mapping user/socket combinations to connection identifiers (ConnIDs).

### CMXsafe Workflow
- CMXsafe System creates a set of CMX-Gateways.
- A device and server is installed with a CMX agent and receives authentication keys.
- The agents establish a secure proxy sessions (SPS) with a CMX-Gateway.
- The CMX-Gateway authenticates the agents, then creates an Identity Socket (ID-sock) and a Mirror Socket (Mir-sock) to preserve identity.
- The device requests a direct socket proxy to connect to a target server.
- Security Contexts (SCs) verify that each communication path is authorized.
- If approved traffic is allowed, the CMX-Gateway forwards traffic between the two parties through the proxy layer.

### Solution Components and Implementation
- CMX Orchestrator:
 - Emulated via Kubernetes control plane which creates and destroys cluster objects dynamically.
 - Each CMX-GW pod uses a ServiceAccount with custom Roles to dynamically create Services.
 - ConfigMaps and entrypoint scripts handle configuration of keys, users, and policy logic.
 - Uses ExternalDNS to automate DNS updates when services are created dynamically.
 - MetalLB acts as a load balancer, allocating external IPs to services.
- CMX Gateways (CMX-GWs):
 - Implemented as containerized pods in Kubernetes using a ReplicaSet.
 - Each pod runs a customized OpenSSH server configured for socket proxying (supporting direct and reverse port forwarding).
 - Stateless design aligns with Kubernetes' container lifecycle and allows horizontal scaling.
 - Automatically authenticated sessions via public key-based SSH connections.
- Agents:
 - IoT devices and servers run OpenSSH clients embedded in Docker containers.
 - Agents initiate Secure Proxy Sessions (SPS) to CMX-GWs using SSH port forwarding.
 - Each agent identifies itself via pre-provisioned keys and MAC-based identity, mapped to custom Linux user accounts.
- Secure Proxy Sessions (SPS):
 - Established via SSH tunnels using direct and reverse forwarding.
 - Maintained persistently using session affinity (Kubernetes sticky sessions).
 - Configured to route IoT traffic (e.g., MQTT) through local ports tunneled to remote sockets in CMX-GWs.
- Identity (ID) and Mirror (Mir) Sockets:
 - Implemented using SSH tunnel source binding.
 - Enabled via forced SSH command execution and per-user configuration blocks in sshd_config.
- Security Contexts (SCs):
 - Achieved using Linux privilege separation within each CMX-GW container.
 - Each IoT device/server has a dedicated user account.
 - SCs are enforced through per-user SSH configurations and isolated session privileges.
 - Session traffic is tied to a user identity, enabling fine-grained control over proxied services.

### Solution Workflow
- CMXsafe System creates a set of CMX-Gateways:
 - Kubernetes cluster spins up CMX-GW pods via a ReplicaSet.
 - Each pod runs a preconfigured OpenSSH server capable of port forwarding.
 - ConfigMaps and volumes populate each pod with keys, user credentials, and startup scripts.
- A device and server is installed with a CMX agent and receives authentication keys:
  - IoT devices and servers are created as Docker containers external to the cluster.
  - Volumes populate each contaner with keys, binaries, and startup scripts
  - On creation, they install and launch OpenSSH clients (Agents).
- The agents establish a secure proxy sessions (SPS) with a CMX-Gateway:
  - The IoT server client opens a reverse port forward from the CMX-GW to itself via the static service domain (e.g., cmxsafe-gw.myservices.local).
  - The IoT device client connects via direct port forwarding to the IoT server’s dynamic service domain (e.g., dynamic-service-server1.myservices.local).
- The CMX-Gateway authenticates the agents, then creates an Identity Socket (ID-sock) and a Mirror Socket (Mir-sock) to preserve identity.
  - A random CMX-GW receives the requests from the IoT server and authenticates the agent using the public key mapped to its Linux user.
  - A forced command runs on successful login to label the pod and dynamically create a service (e.g., dynamic-service-device1) that maps its traffic to the CMX-GW.
  - MetalLB allocates external IPs to the new service and ExternalDNS updates the DNS server to map the service hostname (e.g., dynamic-service-server1.myservices.local) to these IPs.
  - A same CMX-GW receives the requests from the IoT device and authenticates the agent using the public key mapped to its Linux user.
- The device requests a direct socket proxy to connect to a target server.
  - The IoT server opens a reverse port forward from the CMX-GW to itself.
  - The IoT device opens a direct port forward to the IoT server’s dynamic service domain from itself.
  - Kubernetes session affinity ensures traffic from each device consistently routes to the same CMX-GW pod.
- Security Contexts (SCs) verify that each communication path is authorized.
  - SCs isolate inter-process traffic within the pod and prevent unauthorized socket access.
- If approved traffic is allowed, the CMX-Gateway forwards traffic between the two parties through the proxy layer.
  - An MQTT publisher running on the IoT device sends messages to a local port.
  - These messages are then forwarded over the direct port forward to the IoT server’s dynamic service domain.
  - The messages are then received by the CMX-GW and forwarded to the IoT server over the reverse port forward.
  - An MQTT subscriber running on the IoT server receives this message.

## Dependencies and Setup

Our implementation was powered by an Intel Core i7-8700K CPU at 3.7GHz, running a Windows 11 operating system (version 10.0.26100 build 26100). Our host machine has a 64-bit processor architecture, 16GB of RAM, and six logical cores.

### Dependencies

- Docker Desktop:
  - Docker Desktop version 4.38.0 for a Windows 64-bit operating system was used at the time of our testbed setup.
  - We selected the ‘Use WSL 2 instead of Hyper-V’ option on the Configurattion page.
  - Steps:
    - Download and run the installer, Docker Desktop Installer.exe.
    - Follow the instructions on the installation wizard.
    - When the installation is successful, select ‘Close’ to complete the installation process. Then start Docker Desktop.
    - Upon starting Docker Desktop, you may be prompted to install Windows Subsystem for Linux (WSL) 2.  If so, press any key to continue.
    - When the installation is successful, press any key to exit.
    - Accept the Docker Subscription Service Agreement and proceed to Docker Desktop.
- Kubectl:
  - Kubectl client version 1.31.4 and server version 1.32.2 was used at the time of our testbed setup.
  - Steps:
    - Download the kubectl release binary by visiting the Kubernetes release page or executing the following command:
      ```bash
      curl.exe -LO "https://dl.k8s.io/release/v1.33.0/bin/windows/amd64/kubectl.exe"
      ```
    - Prepend the kubectl binary folder to your path environment variable and use the following command to ensure that kubectl was installed successfully:
      ```bash
      kubectl version
      ```
- KinD (Kubernetes-in-Docker):
  - Kind version 0.27.0 for a Windows 64-bit operating system was used at the time of our testbed setup.
  - Steps:
    - Download the release binary, rename it to kind.exe, and place this into your preferred binary installation directory. This can be accomplished on Windows in Powershell using the following commands:
      ```bash
      curl.exe -Lo kind-windows-amd64.exe https://kind.sigs.k8s.io/dl/v0.27.0/kind-windows-amd64
      Move-Item .\kind-windows-amd64.exe c:\some-dir\kind.exe
      ```
    - Add the directory to your path environment variable. Then use the following command to ensure that kind was installed successfully:
      ```bash
      kind version
      ```

### Startup



### Shutdown

Text

## Results

### Deployment of Testbed

Text

### CMXsafe System

Text

### VMs Connect to CMX Gateway

Text

### VMs Communicate via CMX Gateway

Text

## Acknowledgements

This work was completed by Joanne Reilly and Dr. Jorge David de Hoz Diego under the supervision of Dr. Anca Delia Jurcut at University College Dublin.




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
- - Follow these steps to ensure session affinity: https://kind.sigs.k8s.io/docs/user/using-wsl2/#kubernetes-service-with-session-affinity
- Kubectl

## How to run
- Download files.
- Set environment variable CMXSafeProject to location of downloaded files.
- In terminal, go to location of files and run
  .\setup.ps1

