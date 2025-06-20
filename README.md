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
- **CMX Orchestrator:** The management and policy coordination component of CMXSafe.
- **CMX Gateways (CMX-GWs):** Stateless, containerizable socket proxy servers that act as intermediaries for all device-server communications.
- **Agents:** Lightweight client-side proxies on IoT devices and servers that initiate Secure Proxy Sessions (SPSs) using transport layer protocols.
- **Secure Proxy Sessions (SPS):** Encrypted, authenticated sessions that connect agents and gateways.
- **Identity (ID) and Mirror (Mir) Sockets:** Ensure the preservation of device identity within proxied traffic using virtual IPv6 mappings.
- **Security Contexts (SCs):** Enforce inter-process communication policies at the OS level by mapping user/socket combinations to connection identifiers (ConnIDs).

### CMXsafe Workflow
- CMXsafe System creates a set of CMX-Gateways.
- A device and server is installed with a CMX agent and receives authentication keys.
- The agents establish a secure proxy sessions (SPS) with a CMX-Gateway.
- The CMX-Gateway authenticates the agents, then creates an Identity Socket (ID-sock) and a Mirror Socket (Mir-sock) to preserve identity.
- The device requests a direct socket proxy to connect to a target server.
- Security Contexts (SCs) verify that each communication path is authorized.
- If approved traffic is allowed, the CMX-Gateway forwards traffic between the two parties through the proxy layer.

### Solution Components and Implementation
- **CMX Orchestrator:**
  - Emulated via Kubernetes control plane which creates and destroys cluster objects dynamically.
  - Each CMX-GW pod uses a ServiceAccount with custom Roles to dynamically create Services.
  - ConfigMaps and entrypoint scripts handle configuration of keys, users, and policy logic.
  - Uses ExternalDNS to automate DNS updates when services are created dynamically.
  - MetalLB acts as a load balancer, allocating external IPs to services.
- **CMX Gateways (CMX-GWs):**
  - Implemented as containerized pods in Kubernetes using a ReplicaSet.
  - Each pod runs a customized OpenSSH server configured for socket proxying (supporting direct and reverse port forwarding).
  - Stateless design aligns with Kubernetes' container lifecycle and allows horizontal scaling.
  - Automatically authenticated sessions via public key-based SSH connections.
- **Agents:**
  - IoT devices and servers run OpenSSH clients embedded in Docker containers.
  - Agents initiate Secure Proxy Sessions (SPS) to CMX-GWs using SSH port forwarding.
  - Each agent identifies itself via pre-provisioned keys and MAC-based identity, mapped to custom Linux user accounts.
- **Secure Proxy Sessions (SPS):**
  - Established via SSH tunnels using direct and reverse forwarding.
  - Maintained persistently using session affinity (Kubernetes sticky sessions).\
  - Configured to route IoT traffic (e.g., MQTT) through local ports tunneled to remote sockets in CMX-GWs.
- **Identity (ID) and Mirror (Mir) Sockets:**
  - Implemented using SSH tunnel source binding.
  - Enabled via forced SSH command execution and per-user configuration blocks in sshd_config.
- **Security Contexts (SCs):**
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

- **Docker Desktop:**
  - Docker Desktop version 4.38.0 for a Windows 64-bit operating system was used at the time of our testbed setup.
  - We selected the ‘Use WSL 2 instead of Hyper-V’ option on the Configurattion page.
  - Steps:
    - Download and run the installer, Docker Desktop Installer.exe.
    - Follow the instructions on the installation wizard.
    - When the installation is successful, select ‘Close’ to complete the installation process. Then start Docker Desktop.
    - Upon starting Docker Desktop, you may be prompted to install Windows Subsystem for Linux (WSL) 2.  If so, press any key to continue.
    - When the installation is successful, press any key to exit.
    - Accept the Docker Subscription Service Agreement and proceed to Docker Desktop.
- **Kubectl:**
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
- **KinD (Kubernetes-in-Docker):**
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

### Setup

- **Custom Kernel Configuration:**
  - If Docker Desktop uses WSL 2 for its backend, a custom kernel must be created to ensure Kubernetes services with session affinity enabled is accessible.
  - This is because WSL 2 kernels are missing the xt_recent kernel module, which is used by Kube Proxy to implement session affinity.
  - A custom kernel with the xt_recent kernel module enabled can be built using the following commands:
    https://kind.sigs.k8s.io/docs/user/using-wsl2/#kubernetes-service-with-session-affinity
  - Once the kernel is successfully built, create a .wslconfig file in C:\Users\<your-user-name>. This file should contain the following:
    ```bash
    [wsl2]
    kernel=c:\\path\\to\\your\\kernel\\bzImage
    ```
  - Quit Docker Desktop, open an admin PowerShell prompt and run:
    ```bash
    wsl --shutdown.
    ```
  - After waiting for two minutes, restart Docker Desktop and all changes should take effect.
- **Repository Configuration:**
  - Download this GitHub repository.
  - Create an environment variable named CMXSafeProject and set it to the location of the downloaded repository.
  - Inside of the repository is a Windows Powershell script, setup.ps1. Copy this file to the location from which you prefer to execute it from (e.g. C:\Users\<your-user-name>).
- **Deployment:**
  - Restart Docker Desktop and, within the application, open a terminal.
  - Execute the following command:
    ```bash
    Powershell -ExectionPolicy Bypass -File .\<location-of-script>\setup.ps1
    ```
 > [!WARNING]
 > TODO: Script should be digitally signed so that restricted execution policy allows execution of .\setup.ps1

- **Suspension:**
  - To stop the deployment completely, use keys Ctrl and C.
  - Then, execute the following command to remove all objects created during the deployment:
    ```bash
    Powershell -ExectionPolicy Bypass -File .\<location-of-script>\setup.ps1 -Cleanup
    ```
    This allows a clean slate for redeployment.

## Results

After executing the Powershell command, you should see the following lines output to terminal.

### Deployment of Network and Cluster

```bash
Creating Network iot_network...
Creating Kubernetes cluster with Kind...
Creating cluster "cmxsafe" ...
 • Ensuring node image (kindest/node:v1.32.2) 🖼  ...
 ✓ Ensuring node image (kindest/node:v1.32.2) 🖼
 • Preparing nodes 📦 📦   ...
 ✓ Preparing nodes 📦 📦
 • Writing configuration 📜  ...
 ✓ Writing configuration 📜
 • Starting control-plane 🕹️  ...
 ✓ Starting control-plane 🕹️
 • Installing CNI 🔌  ...
 ✓ Installing CNI 🔌
 • Installing StorageClass 💾  ...
 ✓ Installing StorageClass 💾
 • Joining worker nodes 🚜  ...
 ✓ Joining worker nodes 🚜
Set kubectl context to "kind-cmxsafe"
You can now use your cluster with:
kubectl cluster-info --context kind-cmxsafe
Not sure what to do next? 😅  Check out https://kind.sigs.k8s.io/docs/user/quick-start/

namespace/ingress-nginx created
serviceaccount/ingress-nginx created
serviceaccount/ingress-nginx-admission created
role.rbac.authorization.k8s.io/ingress-nginx created
role.rbac.authorization.k8s.io/ingress-nginx-admission created
clusterrole.rbac.authorization.k8s.io/ingress-nginx created
clusterrole.rbac.authorization.k8s.io/ingress-nginx-admission created
rolebinding.rbac.authorization.k8s.io/ingress-nginx created
rolebinding.rbac.authorization.k8s.io/ingress-nginx-admission created
clusterrolebinding.rbac.authorization.k8s.io/ingress-nginx created
clusterrolebinding.rbac.authorization.k8s.io/ingress-nginx-admission created
configmap/ingress-nginx-controller created
service/ingress-nginx-controller created
service/ingress-nginx-controller-admission created
deployment.apps/ingress-nginx-controller created
job.batch/ingress-nginx-admission-create created
job.batch/ingress-nginx-admission-patch created
ingressclass.networking.k8s.io/nginx created
validatingwebhookconfiguration.admissionregistration.k8s.io/ingress-nginx-admission created
```

### Deployment of MetalLB
```bash
namespace/metallb-system created
customresourcedefinition.apiextensions.k8s.io/bfdprofiles.metallb.io created
customresourcedefinition.apiextensions.k8s.io/bgpadvertisements.metallb.io created
customresourcedefinition.apiextensions.k8s.io/bgppeers.metallb.io created
customresourcedefinition.apiextensions.k8s.io/communities.metallb.io created
customresourcedefinition.apiextensions.k8s.io/ipaddresspools.metallb.io created
customresourcedefinition.apiextensions.k8s.io/l2advertisements.metallb.io created
customresourcedefinition.apiextensions.k8s.io/servicel2statuses.metallb.io created
serviceaccount/controller created
serviceaccount/speaker created
role.rbac.authorization.k8s.io/controller created
role.rbac.authorization.k8s.io/pod-lister created
clusterrole.rbac.authorization.k8s.io/metallb-system:controller created
clusterrole.rbac.authorization.k8s.io/metallb-system:speaker created
rolebinding.rbac.authorization.k8s.io/controller created
rolebinding.rbac.authorization.k8s.io/pod-lister created
clusterrolebinding.rbac.authorization.k8s.io/metallb-system:controller created
clusterrolebinding.rbac.authorization.k8s.io/metallb-system:speaker created
configmap/metallb-excludel2 created
secret/metallb-webhook-cert created
service/metallb-webhook-service created
deployment.apps/controller created
daemonset.apps/speaker created
validatingwebhookconfiguration.admissionregistration.k8s.io/metallb-webhook-configuration created
Creating MetalLB...
ipaddresspool.metallb.io/cmxsafe-gateway-pool created
l2advertisement.metallb.io/cmxsafe-gateway-adv created
```

### Creation of Config Maps

```bash
Creating configmap for SSH keys...
configmap/ssh-keys created
configmap/scripts created
```

### Deployment of Cluster Roles and Service Accounts

```bash
Applying Kubernetes configuration for gateway...
serviceaccount/service-creator created
role.rbac.authorization.k8s.io/service-creator-role created
rolebinding.rbac.authorization.k8s.io/service-creator-binding created

```

### Deployment of ExternalDNS

```bash
serviceaccount/external-dns created
clusterrole.rbac.authorization.k8s.io/external-dns created
clusterrolebinding.rbac.authorization.k8s.io/external-dns created
deployment.apps/external-dns created
```

### Deployment of CMXsafe Gateway ReplicaSet and Load Balancing Service

```bash
deployment.apps/cmxsafe-gw created
Applying Kubernetes service for gateway...
service/cmxsafe-gw created
```

### Deployment of IoT Server, IoT Device and BIND9 DNS

```bash
Building and starting Docker containers...

[+] Building 1.9s (23/23) FINISHED                                                                                                                       docker:desktop-linux
 => [iot_server internal] load build definition from Dockerfile                                                                                                          0.0s
 => => transferring dockerfile: 534B                                                                                                                                     0.0s 
 => [iot_device internal] load metadata for docker.io/library/ubuntu:latest                                                                                              0.1s 
 => [iot_server internal] load .dockerignore                                                                                                                             0.0s 
 => => transferring context: 2B                                                                                                                                          0.0s 
 => [iot_device 1/5] FROM docker.io/library/ubuntu:latest@sha256:72297848456d5d37d1262630108ab308d3e9ec7ed1c3286a32fe09856619a782                                        1.1s 
 => => resolve docker.io/library/ubuntu:latest@sha256:72297848456d5d37d1262630108ab308d3e9ec7ed1c3286a32fe09856619a782                                                   0.0s 
 => [iot_server internal] load build context                                                                                                                             0.0s 
 => => transferring context: 30B                                                                                                                                         0.0s
 => [iot_server auth] library/ubuntu:pull token for registry-1.docker.io                                                                                                 0.0s
 => CACHED [iot_server 2/7] RUN apt update && apt install -y iputils-ping iputils-arping dnsutils openssh-client mosquitto mosquitto-clients net-tools dos2unix tcpdump  0.0s 
 => CACHED [iot_server 3/7] RUN echo "listener 1883" >> /etc/mosquitto/mosquitto.conf                                                                                    0.0s 
 => CACHED [iot_server 4/7] RUN echo "allow_anonymous true" >> /etc/mosquitto/mosquitto.conf                                                                             0.0s 
 => CACHED [iot_server 5/7] COPY start.sh /start.sh                                                                                                                      0.0s 
 => CACHED [iot_server 6/7] RUN chmod +x /start.sh                                                                                                                       0.0s 
 => CACHED [iot_server 7/7] RUN dos2unix /start.sh && chmod +x /start.sh                                                                                                 0.0s 
 => [iot_server] exporting to image                                                                                                                                      0.1s 
 => => exporting layers                                                                                                                                                  0.0s 
 => => exporting manifest sha256:3edcf6c8f48709da1e4ec8142f21be3b11f104838d29841fc031b1340ee34f87                                                                        0.0s
 => => exporting config sha256:0af5052c2c36d27ee286150e1d7e2affbd4872544176f94d363b3bb105053170                                                                          0.0s
 => => exporting attestation manifest sha256:3650b2d14ac209fbe683d6401eecb8f36dbc0b1307a162c497160bddcf60e3a8                                                            0.0s 
 => => exporting manifest list sha256:30654798fc30d280476157f37a7cafbf39eada188a20571af18b8448d011049d                                                                   0.0s 
 => => naming to docker.io/library/iot_project-iot_server:latest                                                                                                         0.0s 
 => => unpacking to docker.io/library/iot_project-iot_server:latest                                                                                                      0.0s 
 => [iot_server] resolving provenance for metadata file                                                                                                                  0.0s 
 => [iot_device internal] load build definition from Dockerfile                                                                                                          0.0s 
 => => transferring dockerfile: 386B                                                                                                                                     0.0s 
 => [iot_device internal] load .dockerignore                                                                                                                             0.0s 
 => => transferring context: 2B                                                                                                                                          0.0s 
 => [iot_device internal] load build context                                                                                                                             0.0s 
 => => transferring context: 30B                                                                                                                                         0.0s 
 => CACHED [iot_device 2/5] RUN apt update && apt install -y iputils-ping iputils-arping dnsutils openssh-client mosquitto-clients dos2unix net-tools tcpdump iproute2   0.0s 
 => CACHED [iot_device 3/5] COPY start.sh /start.sh                                                                                                                      0.0s 
 => CACHED [iot_device 4/5] RUN chmod +x /start.sh                                                                                                                       0.0s 
 => CACHED [iot_device 5/5] RUN dos2unix /start.sh && chmod +x /start.sh                                                                                                 0.0s 
 => [iot_device] exporting to image                                                                                                                                      0.1s 
 => => exporting layers                                                                                                                                                  0.0s 
 => => exporting manifest sha256:bfde8559d2acc98f711e0d19c108ab4fbb3f765f43117bf85a236c469e5ccccb                                                                        0.0s 
 => => exporting config sha256:0fc2c471ac44f9e39ff37a726b9cd8f0378064f052bf577c70f6ac2690fe4da7                                                                          0.0s 
 => => exporting attestation manifest sha256:af52a48da6618b5ae10ec00e44a6a2e20d8df32f059a987908d2a91b53f66908                                                            0.0s 
 => => exporting manifest list sha256:d78eff686f0f36f735065484c702e7342ec0a74786121f3a97425a48d497fe34                                                                   0.0s 
 => => naming to docker.io/library/iot_project-iot_device:latest                                                                                                         0.0s 
 => => unpacking to docker.io/library/iot_project-iot_device:latest                                                                                                      0.0s 
 => [iot_device] resolving provenance for metadata file                                                                                                                  0.0s 
[+] Running 5/5
 ✔ iot_device                    Built                                                                                                                                   0.0s 
 ✔ iot_server                    Built                                                                                                                                   0.0s 
 ✔ Container iot_server          Created                                                                                                                                 0.2s 
 ✔ Container iot_project-bind-1  Created                                                                                                                                 0.2s 
 ✔ Container iot_device          Created                                                                                                                                 0.1s 
Attaching to iot_device, bind-1, iot_server
```

### BIND9 DNS Runtime

```bash
bind-1      | Starting named...
bind-1      | exec /usr/sbin/named -u "root" -g ""
bind-1      | 20-Jun-2025 16:21:13.387 starting BIND 9.18.30-0ubuntu0.22.04.2-Ubuntu (Extended Support Version) <id:>                                                         
bind-1      | 20-Jun-2025 16:21:13.387 running on Linux x86_64 5.15.146.1-microsoft-standard-WSL2+ #1 SMP Wed Apr 23 16:39:49 IST 2025
bind-1      | 20-Jun-2025 16:21:13.387 built with  '--build=x86_64-linux-gnu' '--prefix=/usr' '--includedir=${prefix}/include' '--mandir=${prefix}/share/man' '--infodir=${prefix}/share/info' '--sysconfdir=/etc' '--localstatedir=/var' '--disable-option-checking' '--disable-silent-rules' '--libdir=${prefix}/lib/x86_64-linux-gnu' '--runstatedir=/run' '--disable-maintainer-mode' '--disable-dependency-tracking' '--libdir=/usr/lib/x86_64-linux-gnu' '--sysconfdir=/etc/bind' '--with-python=python3' '--localstatedir=/' '--enable-threads' '--enable-largefile' '--with-libtool' '--enable-shared' '--disable-static' '--with-gost=no' '--with-openssl=/usr' '--with-gssapi=yes' '--with-libidn2' '--with-json-c' '--with-lmdb=/usr' '--with-gnu-ld' '--with-maxminddb' '--with-atf=no' '--enable-ipv6' '--enable-rrl' '--enable-filter-aaaa' '--disable-native-pkcs11' 'build_alias=x86_64-linux-gnu' 'CFLAGS=-g -O2 -ffile-prefix-map=/build/bind9-AB1uwX/bind9-9.18.30=. -flto=auto -ffat-lto-objects -flto=auto -ffat-lto-objects -fstack-protector-strong -Wformat -Werror=format-security -fno-strict-aliasing -fno-delete-null-pointer-checks -DNO_VERSION_DATE -DDIG_SIGCHASE' 'LDFLAGS=-Wl,-Bsymbolic-functions -flto=auto -ffat-lto-objects -flto=auto -Wl,-z,relro -Wl,-z,now' 'CPPFLAGS=-Wdate-time -D_FORTIFY_SOURCE=2'
bind-1      | 20-Jun-2025 16:21:13.387 running as: named -u root -g
bind-1      | 20-Jun-2025 16:21:13.387 compiled by GCC 11.4.0
bind-1      | 20-Jun-2025 16:21:13.387 compiled with OpenSSL version: OpenSSL 3.0.2 15 Mar 2022                                                                               
bind-1      | 20-Jun-2025 16:21:13.387 linked to OpenSSL version: OpenSSL 3.0.2 15 Mar 2022                                                                                   
bind-1      | 20-Jun-2025 16:21:13.387 compiled with libuv version: 1.43.0
bind-1      | 20-Jun-2025 16:21:13.387 linked to libuv version: 1.43.0                                                                                                        
bind-1      | 20-Jun-2025 16:21:13.387 compiled with libxml2 version: 2.9.13                                                                                                  
bind-1      | 20-Jun-2025 16:21:13.387 linked to libxml2 version: 20913
bind-1      | 20-Jun-2025 16:21:13.387 compiled with json-c version: 0.15                                                                                                     
bind-1      | 20-Jun-2025 16:21:13.387 linked to json-c version: 0.15
bind-1      | 20-Jun-2025 16:21:13.387 compiled with zlib version: 1.2.11                                                                                                     
bind-1      | 20-Jun-2025 16:21:13.387 linked to zlib version: 1.2.11
bind-1      | 20-Jun-2025 16:21:13.387 ----------------------------------------------------                                                                                   
bind-1      | 20-Jun-2025 16:21:13.387 BIND 9 is maintained by Internet Systems Consortium,                                                                                   
bind-1      | 20-Jun-2025 16:21:13.387 Inc. (ISC), a non-profit 501(c)(3) public-benefit 
bind-1      | 20-Jun-2025 16:21:13.387 corporation.  Support and training for BIND 9 are                                                                                      
bind-1      | 20-Jun-2025 16:21:13.387 available at https://www.isc.org/support
bind-1      | 20-Jun-2025 16:21:13.387 ----------------------------------------------------
bind-1      | 20-Jun-2025 16:21:13.387 found 12 CPUs, using 12 worker threads                                                                                                 
bind-1      | 20-Jun-2025 16:21:13.387 using 12 UDP listeners per interface
bind-1      | 20-Jun-2025 16:21:13.407 DNSSEC algorithms: RSASHA1 NSEC3RSASHA1 RSASHA256 RSASHA512 ECDSAP256SHA256 ECDSAP384SHA384 ED25519 ED448                              
bind-1      | 20-Jun-2025 16:21:13.407 DS algorithms: SHA-1 SHA-256 SHA-384
bind-1      | 20-Jun-2025 16:21:13.407 HMAC algorithms: HMAC-MD5 HMAC-SHA1 HMAC-SHA224 HMAC-SHA256 HMAC-SHA384 HMAC-SHA512                                                    
bind-1      | 20-Jun-2025 16:21:13.407 TKEY mode 2 support (Diffie-Hellman): yes                                                                                              
bind-1      | 20-Jun-2025 16:21:13.407 TKEY mode 3 support (GSS-API): yes
bind-1      | 20-Jun-2025 16:21:13.407 the initial working directory is '/'                                                                                                   
bind-1      | 20-Jun-2025 16:21:13.407 loading configuration from '/etc/bind/named.conf'                                                                                      
bind-1      | 20-Jun-2025 16:21:13.437 unable to open '/etc/bind/bind.keys'; using built-in keys instead
bind-1      | 20-Jun-2025 16:21:13.437 looking for GeoIP2 databases in '/usr/share/GeoIP'                                                                                     
bind-1      | 20-Jun-2025 16:21:13.437 using default UDP/IPv4 port range: [32768, 60999]
bind-1      | 20-Jun-2025 16:21:13.437 using default UDP/IPv6 port range: [32768, 60999]
bind-1      | 20-Jun-2025 16:21:13.497 listening on IPv4 interface lo, 127.0.0.1#53                                                                                           
bind-1      | 20-Jun-2025 16:21:13.497 listening on IPv4 interface eth0, 192.168.1.29#53
bind-1      | 20-Jun-2025 16:21:13.497 IPv6 socket API is incomplete; explicitly binding to each IPv6 address separately                                                      
bind-1      | 20-Jun-2025 16:21:13.497 listening on IPv6 interface lo, ::1#53                                                                                                 
bind-1      | 20-Jun-2025 16:21:13.497 listening on IPv6 interface eth0, fd00::4#53                                                                                           
bind-1      | 20-Jun-2025 16:21:13.497 listening on IPv6 interface eth0, fe80::42:c0ff:fea8:11d%38#53                                                                         
bind-1      | 20-Jun-2025 16:21:13.507 Could not open '//run/named/named.pid'.
bind-1      | 20-Jun-2025 16:21:13.507 Please check file and directory permissions or reconfigure the filename.                                                               
bind-1      | 20-Jun-2025 16:21:13.507 could not open file '//run/named/named.pid': Permission denied
bind-1      | 20-Jun-2025 16:21:13.507 generating session key for dynamic DNS                                                                                                 
bind-1      | 20-Jun-2025 16:21:13.507 Could not open '//run/named/session.key'.
bind-1      | 20-Jun-2025 16:21:13.507 Please check file and directory permissions or reconfigure the filename.                                                               
bind-1      | 20-Jun-2025 16:21:13.507 could not open file '//run/named/session.key': Permission denied                                                                       
bind-1      | 20-Jun-2025 16:21:13.507 could not create //run/named/session.key
bind-1      | 20-Jun-2025 16:21:13.507 failed to generate session key for dynamic DNS: permission denied                                                                      
bind-1      | 20-Jun-2025 16:21:13.507 sizing zone task pool based on 1 zones                                                                                                 
bind-1      | 20-Jun-2025 16:21:13.507 zone 'myservices.local' allows unsigned updates from remote hosts, which is insecure
bind-1      | 20-Jun-2025 16:21:13.507 none:99: 'max-cache-size 90%' - setting to 7112MB (out of 7903MB)                                                                      
bind-1      | 20-Jun-2025 16:21:13.507 using built-in root key for view _default
bind-1      | 20-Jun-2025 16:21:13.507 set up managed keys zone for view _default, file 'managed-keys.bind'                                                                   
bind-1      | 20-Jun-2025 16:21:13.507 automatic empty zone: 10.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.507 automatic empty zone: 16.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.507 automatic empty zone: 17.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 18.172.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 19.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 20.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 21.172.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 22.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 23.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 24.172.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 25.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 26.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 27.172.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 28.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 29.172.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 30.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 31.172.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 168.192.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 64.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 65.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 66.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 67.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 68.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 69.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 70.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 71.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 72.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 73.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 74.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 75.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 76.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 77.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 78.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 79.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 80.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 81.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 82.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 83.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 84.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 85.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 86.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 87.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 88.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 89.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 90.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 91.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 92.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 93.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 94.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 95.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 96.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 97.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 98.100.IN-ADDR.ARPA                                                                                              
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 99.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 100.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 101.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 102.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 103.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 104.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 105.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 106.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 107.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 108.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 109.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 110.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 111.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 112.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 113.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 114.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 115.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 116.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 117.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 118.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 119.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 120.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 121.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 122.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 123.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 124.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 125.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 126.100.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 127.100.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 0.IN-ADDR.ARPA                                                                                                   
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 127.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 254.169.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 2.0.192.IN-ADDR.ARPA                                                                                             
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 100.51.198.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 113.0.203.IN-ADDR.ARPA                                                                                           
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 255.255.255.255.IN-ADDR.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.IP6.ARPA                                         
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.IP6.ARPA                                         
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: D.F.IP6.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 8.E.F.IP6.ARPA                                                                                                   
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 9.E.F.IP6.ARPA                                                                                                   
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: A.E.F.IP6.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: B.E.F.IP6.ARPA                                                                                                   
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: 8.B.D.0.1.0.0.2.IP6.ARPA
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: EMPTY.AS112.ARPA                                                                                                 
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: HOME.ARPA                                                                                                        
bind-1      | 20-Jun-2025 16:21:13.517 automatic empty zone: RESOLVER.ARPA
bind-1      | 20-Jun-2025 16:21:13.527 configuring command channel from '/etc/bind/rndc.key'                                                                                  
bind-1      | 20-Jun-2025 16:21:13.527 command channel listening on 127.0.0.1#953                                                                                             
bind-1      | 20-Jun-2025 16:21:13.527 configuring command channel from '/etc/bind/rndc.key'
bind-1      | 20-Jun-2025 16:21:13.607 command channel listening on ::1#953                                                                                                   
bind-1      | 20-Jun-2025 16:21:13.607 not using config file logging statement for logging due to -g option                                                                   
bind-1      | 20-Jun-2025 16:21:13.607 managed-keys-zone: loaded serial 0
bind-1      | 20-Jun-2025 16:21:13.667 zone myservices.local/IN: loaded serial 2025041704                                                                                     
bind-1      | 20-Jun-2025 16:21:13.667 all zones loaded                                                                                                                       
bind-1      | 20-Jun-2025 16:21:13.677 running
bind-1      | 20-Jun-2025 16:21:13.787 managed-keys-zone: Initializing automatic trust anchor management for zone '.'; DNSKEY ID 20326 is now trusted, waiving the normal 30-day waiting period.                                                                                                                                                            
bind-1      | 20-Jun-2025 16:21:13.787 managed-keys-zone: Initializing automatic trust anchor management for zone '.'; DNSKEY ID 38696 is now trusted, waiving the normal 30-day waiting period.
bind-1      | 20-Jun-2025 16:32:08.491 client @0x7f2088006d48 192.168.1.3#40086: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"                                                                                 
bind-1      | 20-Jun-2025 16:32:08.491 client @0x7f2088006d48 192.168.1.3#40086: updating zone 'myservices.local/IN': adding an RR at 'a-cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"
bind-1      | 20-Jun-2025 16:32:08.491 client @0x7f2088006d48 192.168.1.3#40086: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' A 192.168.1.240

bind-1      | 20-Jun-2025 16:31:43.421 running
bind-1      | 20-Jun-2025 16:31:43.481 managed-keys-zone: Initializing automatic trust anchor management for zone '.'; DNSKEY ID 20326 is now trusted, waiving the normal 30-day waiting period.
bind-1      | 20-Jun-2025 16:31:43.481 managed-keys-zone: Initializing automatic trust anchor management for zone '.'; DNSKEY ID 38696 is now trusted, waiving the normal 30-day waiting period.
bind-1      | 20-Jun-2025 16:32:08.491 client @0x7f2088006d48 192.168.1.3#40086: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' A 192.168.1.240
bind-1      | 20-Jun-2025 16:32:08.491 client @0x7f2088006d48 192.168.1.3#40086: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"
bind-1      | 20-Jun-2025 16:32:08.491 client @0x7f2088006d48 192.168.1.3#40086: updating zone 'myservices.local/IN': adding an RR at 'a-cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"
```

### IoT Server and IoT Device Runtime

```bash
iot_server  | CMX Agent a23ff868ec8f Running...
iot_server  | Attempting to connect via SSH...
iot_server  | Pseudo-terminal will not be allocated because stdin is not a terminal.
iot_server  | ssh: connect to host cmxsafe-gw.myservices.local port 22: Connection refused
iot_device  | CMX Agent 025f9a3db712 Running...
iot_device  | Attempting to connect via SSH...
iot_device  | ssh: connect to host dynamic-service-a23ff868ec8f.myservices.local port 22: No route to host
iot_server  | Pseudo-terminal will not be allocated because stdin is not a terminal.
iot_server  | ssh: connect to host cmxsafe-gw.myservices.local port 22: Connection refused
iot_device  | ssh: connect to host dynamic-service-a23ff868ec8f.myservices.local port 22: No route to host                                                                    
bind-1      | 20-Jun-2025 16:33:08.483 client @0x7f2068004d18 192.168.1.3#53450: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' A 192.168.1.240
bind-1      | 20-Jun-2025 16:33:08.483 client @0x7f2068004d18 192.168.1.3#53450: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"                                                                                 
bind-1      | 20-Jun-2025 16:33:08.483 client @0x7f2068004d18 192.168.1.3#53450: updating zone 'myservices.local/IN': adding an RR at 'a-cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"                                                                               
iot_server  | Pseudo-terminal will not be allocated because stdin is not a terminal.
iot_server  | Warning: Permanently added 'cmxsafe-gw.myservices.local' (ED25519) to the list of known hosts.
iot_device  | ssh: connect to host dynamic-service-a23ff868ec8f.myservices.local port 22: No route to host
iot_server  | service/dynamic-service-a23ff868ec8f created
iot_server  | SSH tunnel established successfully!
iot_server  | Starting MQTT subscriber...
iot_device  | Warning: Permanently added 'dynamic-service-a23ff868ec8f.myservices.local' (ED25519) to the list of known hosts.
iot_device  | SSH tunnel established successfully!
iot_device  | Starting MQTT publisher...
iot_server  | Hello from IoT Device
iot_server  | Hello from IoT Device                                                                                                                                           
iot_server  | Hello from IoT Device
```

### Suspension

```bash
Gracefully stopping... (press Ctrl+C again to force)
[+] Stopping 3/3
 ✔ Container iot_project-bind-1  Stopped                                                                                                                                 1.5s 
 ✔ Container iot_device          Stopped                                                                                                                                10.4s 
 ✔ Container iot_server          Stopped                                                                                                                                11.1s 
canceled
```

### Clean Up

```bash
Stopping Docker containers...
time="2025-06-20T17:36:40+01:00" level=warning msg="C:\\Users\\joann\\CMXSafeProject\\iot_project\\docker-compose.yml: the attribute `version` is obsolete, it will be ignored, please remove it to avoid potential confusion"
[+] Running 3/3
 ✔ Container iot_device          Removed                                                                                                                                 0.0s 
 ✔ Container iot_project-bind-1  Removed                                                                                                                                 0.1s 
 ✔ Container iot_server          Removed                                                                                                                                 0.1s 
configmap "ssh-keys" deleted
configmap "scripts" deleted
Deleting Kind cluster...
Deleting cluster "cmxsafe" ...
Deleted nodes: ["cmxsafe-worker" "cmxsafe-control-plane"]
iot_network
Deleting file: C:\Users\joann\CMXSafeProject\iot_project\keys\iot-device\known_hosts
Successfully deleted: C:\Users\joann\CMXSafeProject\iot_project\keys\iot-device\known_hosts
File not found: C:\Users\joann\CMXSafeProject\iot_project\keys\iot-device\known_hosts.old
Deleting file: C:\Users\joann\CMXSafeProject\iot_project\keys\iot-server\known_hosts
Successfully deleted: C:\Users\joann\CMXSafeProject\iot_project\keys\iot-server\known_hosts
File not found: C:\Users\joann\CMXSafeProject\iot_project\keys\iot-server\known_hosts.old
Cleanup completed successfully!
```

## Acknowledgements

This work was completed by Joanne Reilly and Dr. Jorge David de Hoz Diego under the supervision of Dr. Anca Delia Jurcut in the School of Computer Science at University College Dublin, Belfield, Dublin 4, Ireland.

