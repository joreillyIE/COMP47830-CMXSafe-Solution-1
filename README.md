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
- **Helm:**
  - Helm version 3.18.4, compiled using Go version 1.24.4, was used at the time of our testbed setup. We installed Helm with Chocolatey version 2.4.3.
  - Steps:
    - Download the Chocolatey release binary by visiting the Chocolatey release page or executing the following command:
      ```bash
      curl.exe -LO "https://github.com/chocolatey/choco/releases/download/2.4.3/chocolatey-2.4.3.0.msi"
      ```
    - Place this file into your preferred binary installation directory before installation. This can be accomplished on Windows in Powershell using the following commands:
      ```bash
      Move-Item .\chocolatey-2.4.3.0.msi c:\some-dir\chocolatey.msi
      chocolatey-2.4.3.0.msi
      ```
    - Follow the instructions on the installation wizard. When the installation process completes, use the following command to ensure that Chocolatey was installed successfully:
      ```bash
      choco -?
      ```
    - Install Helm and verify installation using the following commands:
      ```bash
      choco install kubernetes-helm -y --version=3.18.4
      helm version
      ```

### Setup

- **Custom Kernel Configuration:**
  - If Docker Desktop uses WSL 2 for its backend, a custom kernel must be created to ensure Kubernetes services with session affinity enabled is accessible.
  - This is because WSL 2 kernels are missing the xt_recent kernel module, which is used by Kube Proxy to implement session affinity.
  - A custom kernel with the xt_recent kernel module enabled can be built using the following commands:
    https://kind.sigs.k8s.io/docs/user/using-wsl2/#kubernetes-service-with-session-affinity
  - Included in the repository is a ready-built custom kernel image used during implementation and built following the steps above.
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
e7fc8a2f954e04a58b24878763b7b639446f6cf430d122abe438d4643dbb5d23
Creating Kubernetes cluster with KinD...
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
node/cmxsafe-control-plane condition met
node/cmxsafe-worker condition me
```

### Deployment of Tetragon
```bash
"cilium" already exists with the same configuration, skipping
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "cilium" chart repository
Update Complete. ⎈Happy Helming!⎈
NAME: tetragon
LAST DEPLOYED: Tue Aug 12 14:24:38 2025
NAMESPACE: kube-system
STATUS: deployed
REVISION: 1
TEST SUITE: None
Waiting for daemon set "tetragon" rollout to finish: 0 of 2 updated pods are available...
Waiting for daemon set "tetragon" rollout to finish: 1 of 2 updated pods are available...
daemon set "tetragon" successfully rolled out
```

### Deployment of K8S Go Client
```bash
Setting up K8S Go Client...
[+] Building 1.3s (14/14) FINISHED                                                                                                                       docker:desktop-linux
 => [internal] load build definition from Dockerfile                                                                                                                     0.0s
 => => transferring dockerfile: 216B                                                                                                                                     0.0s 
 => [internal] load metadata for docker.io/library/golang:1.24.5-alpine3.22                                                                                              1.0s 
 => [internal] load metadata for docker.io/library/debian:latest                                                                                                         1.0s 
 => [auth] library/debian:pull token for registry-1.docker.io                                                                                                            0.0s
 => [auth] library/golang:pull token for registry-1.docker.io                                                                                                            0.0s
 => [internal] load .dockerignore                                                                                                                                        0.0s
 => => transferring context: 2B                                                                                                                                          0.0s 
 => [builder 1/4] FROM docker.io/library/golang:1.24.5-alpine3.22@sha256:daae04ebad0c21149979cd8e9db38f565ecefd8547cf4a591240dc1972cf1399                                0.0s 
 => => resolve docker.io/library/golang:1.24.5-alpine3.22@sha256:daae04ebad0c21149979cd8e9db38f565ecefd8547cf4a591240dc1972cf1399                                        0.0s 
 => [internal] load build context                                                                                                                                        0.0s 
 => => transferring context: 142B                                                                                                                                        0.0s 
 => [stage-1 1/2] FROM docker.io/library/debian:latest@sha256:b6507e340c43553136f5078284c8c68d86ec8262b1724dde73c325e8d3dcdeba                                           0.0s 
 => => resolve docker.io/library/debian:latest@sha256:b6507e340c43553136f5078284c8c68d86ec8262b1724dde73c325e8d3dcdeba                                                   0.0s 
 => CACHED [builder 2/4] WORKDIR /app                                                                                                                                    0.0s 
 => CACHED [builder 3/4] COPY . .                                                                                                                                        0.0s 
 => CACHED [builder 4/4] RUN GOOS=linux go build -o app .                                                                                                                0.0s 
 => CACHED [stage-1 2/2] COPY --from=builder /app/app /app                                                                                                               0.0s 
 => exporting to image                                                                                                                                                   0.1s
 => => exporting layers                                                                                                                                                  0.0s 
 => => exporting manifest sha256:6c1175bab9aba1364d11baa279d0c09023f0591b997604ef33723ba99243d168                                                                        0.0s 
 => => exporting config sha256:1e81ce94ef483a723d530dcf10ce9bd418e3ea8d46e9a77becaba8aa152d4a7b                                                                          0.0s 
 => => exporting attestation manifest sha256:66f6525f909d278d71edd7e447f854b1a06c8fc5188d31a1a90fab7dd7228902                                                            0.0s 
 => => exporting manifest list sha256:8110eafe20508aca4975e0711030ac1d37840777e726f67bf4b29b5557bc270c                                                                   0.0s 
 => => naming to docker.io/library/myclient:dev                                                                                                                          0.0s 
 => => unpacking to docker.io/library/myclient:dev                                                                                                                       0.0s 

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/lrq7qxa1tqtxz4yin35r16jeh
Image: "myclient:dev" with ID "sha256:8110eafe20508aca4975e0711030ac1d37840777e726f67bf4b29b5557bc270c" not yet present on node "cmxsafe-worker", loading...
Image: "myclient:dev" with ID "sha256:8110eafe20508aca4975e0711030ac1d37840777e726f67bf4b29b5557bc270c" not yet present on node "cmxsafe-control-plane", loading...
serviceaccount/service-creator created
clusterrole.rbac.authorization.k8s.io/service-creator-clusters created
clusterrolebinding.rbac.authorization.k8s.io/service-creator-bindings created
pod/client-go created
```

### Deployment of MetalLB
```bash
Setting up MetalLB...
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
customresourcedefinition.apiextensions.k8s.io/ipaddresspools.metallb.io condition met
customresourcedefinition.apiextensions.k8s.io/l2advertisements.metallb.io condition met
Waiting for deployment "controller" rollout to finish: 0 of 1 updated replicas are available...
deployment "controller" successfully rolled out
Waiting for daemon set "speaker" rollout to finish: 0 of 2 updated pods are available...
Waiting for daemon set "speaker" rollout to finish: 1 of 2 updated pods are available...
daemon set "speaker" successfully rolled out
ipaddresspool.metallb.io/cmxsafe-gateway-pool created
l2advertisement.metallb.io/cmxsafe-gateway-adv created
```

### Deployment of ExternalDNS

```bash
Setting up External DNS...
serviceaccount/external-dns created
clusterrole.rbac.authorization.k8s.io/external-dns created
clusterrolebinding.rbac.authorization.k8s.io/external-dns created
deployment.apps/external-dns created
```

### Creation of Secrets

```bash
Creating Secrets for SSH Host Keys and Configurations...
secret/cmxsafe-sshd-config created
secret/cmxsafe-host-keys created
Creating Secrets for IoT Users and Authorized Keys...
secret/iot-users created
```

### Deployment of CMXsafe Gateway ReplicaSet and Load Balancing Service

```bash
Applying Kubernetes configuration for gateways...
[+] Building 1.3s (19/19) FINISHED                                                                                                                       docker:desktop-linux
 => [internal] load build definition from Dockerfile                                                                                                                     0.0s
 => => transferring dockerfile: 1.15kB                                                                                                                                   0.0s 
 => [internal] load metadata for docker.io/library/ubuntu:24.04                                                                                                          1.0s 
 => [auth] library/ubuntu:pull token for registry-1.docker.io                                                                                                            0.0s
 => [internal] load .dockerignore                                                                                                                                        0.0s
 => => transferring context: 2B                                                                                                                                          0.0s 
 => [internal] load build context                                                                                                                                        0.1s 
 => => transferring context: 68.17kB                                                                                                                                     0.0s 
 => [builder 1/7] FROM docker.io/library/ubuntu:24.04@sha256:a08e551cb33850e4740772b38217fc1796a66da2506d312abe51acda354ff061                                            0.0s 
 => => resolve docker.io/library/ubuntu:24.04@sha256:a08e551cb33850e4740772b38217fc1796a66da2506d312abe51acda354ff061                                                    0.0s 
 => CACHED [stage-1 2/7] RUN apt-get update && apt-get install -y --no-install-recommends zlib1g libssl3 && rm -rf /var/lib/apt/lists/*                                  0.0s
 => CACHED [builder 2/7] RUN apt-get update && apt-get install -y build-essential zlib1g-dev libssl-dev libedit-dev pkg-config autoconf automake libtool dos2unix        0.0s 
 => CACHED [builder 3/7] COPY openssh/openssh-portable-V_9_2_P1 /src/openssh                                                                                             0.0s 
 => CACHED [builder 4/7] WORKDIR /src/openssh                                                                                                                            0.0s 
 => CACHED [builder 5/7] RUN autoreconf && ./configure --prefix=/usr --sysconfdir=/etc/ssh --with-privsep-path=/var/lib/sshd --without-zlib-version-check && make -j$(n  0.0s 
 => CACHED [builder 6/7] COPY createservice.sh /tmp/createservice.sh                                                                                                     0.0s 
 => CACHED [builder 7/7] RUN chmod +x /tmp/createservice.sh && dos2unix /tmp/createservice.sh                                                                            0.0s 
 => CACHED [stage-1 3/7] COPY --from=builder /tmp/install/ /                                                                                                             0.0s 
 => CACHED [stage-1 4/7] RUN mkdir -p /var/run/sshd /var/lib/sshd                                                                                                        0.0s
 => CACHED [stage-1 5/7] RUN useradd -r -M -s /usr/sbin/nologin sshd                                                                                                     0.0s 
 => CACHED [stage-1 6/7] COPY --from=builder /tmp/createservice.sh /etc/createservice.sh                                                                                 0.0s 
 => CACHED [stage-1 7/7] RUN chmod +x /etc/createservice.sh                                                                                                              0.0s 
 => exporting to image                                                                                                                                                   0.1s 
 => => exporting layers                                                                                                                                                  0.0s 
 => => exporting manifest sha256:ba3cfbd9c4378fd2e8820b7b221055ae4a5d996b54c166189f7b666358ba2043                                                                        0.0s 
 => => exporting config sha256:0b0a081684ea3c2c7701563ea1eb7ece507b9e6e6b4052cf39a8b65ba2661233                                                                          0.0s 
 => => exporting attestation manifest sha256:e2fd7ca1931c9937828ef2a9d896da9852491d106a6d9604e33db82359298309                                                            0.0s 
 => => exporting manifest list sha256:e5a832c32838dc2e2f3066ee654a2c6216ac8861127b3175cb6f8032eadccc91                                                                   0.0s 
 => => naming to docker.io/library/gateway:dev                                                                                                                           0.0s 
 => => unpacking to docker.io/library/gateway:dev                                                                                                                        0.0s 

View build details: docker-desktop://dashboard/build/desktop-linux/desktop-linux/09ep890uende3fx25wj5aez2d
Image: "gateway:dev" with ID "sha256:e5a832c32838dc2e2f3066ee654a2c6216ac8861127b3175cb6f8032eadccc91" not yet present on node "cmxsafe-worker", loading...
Image: "gateway:dev" with ID "sha256:e5a832c32838dc2e2f3066ee654a2c6216ac8861127b3175cb6f8032eadccc91" not yet present on node "cmxsafe-control-plane", loading...
deployment.apps/cmxsafe-gw created
Applying Kubernetes service for gateways...
service/cmxsafe-gw created
```

### Deployment of IoT Server, IoT Device and BIND9 DNS

```bash
Building and starting Docker containers...
time="2025-08-12T14:26:27+01:00" level=warning msg="C:\\Users\\joann\\CMXSafeProject\\endpoints\\docker-compose.yml: the attribute `version` is obsolete, it will be ignored, please remove it to avoid potential confusion"
[+] Building 3.0s (37/37) FINISHED                                                                                                                       docker:desktop-linux
 => [iot_server internal] load build definition from Dockerfile                                                                                                          0.0s
 => => transferring dockerfile: 1.34kB                                                                                                                                   0.0s 
 => [iot_device_2 internal] load metadata for docker.io/library/ubuntu:24.04                                                                                             0.8s 
 => [iot_server internal] load .dockerignore                                                                                                                             0.0s
 => => transferring context: 2B                                                                                                                                          0.0s 
 => [iot_device_1 1/5] FROM docker.io/library/ubuntu:24.04@sha256:a08e551cb33850e4740772b38217fc1796a66da2506d312abe51acda354ff061                                       0.2s 
 => => resolve docker.io/library/ubuntu:24.04@sha256:a08e551cb33850e4740772b38217fc1796a66da2506d312abe51acda354ff061                                                    0.1s 
 => [iot_server internal] load build context                                                                                                                             0.3s 
 => => transferring context: 68.16kB                                                                                                                                     0.3s
 => CACHED [iot_server stage-1 2/8] RUN apt-get update && apt-get install -y --no-install-recommends zlib1g libssl3 iproute2 mosquitto mosquitto-clients && rm -rf /var  0.0s
 => CACHED [iot_server builder 2/7] RUN apt-get update && apt-get install -y build-essential zlib1g-dev libssl-dev libedit-dev pkg-config autoconf automake libtool dos  0.0s 
 => CACHED [iot_server builder 3/7] COPY openssh/openssh-portable-V_9_2_P1 /src/openssh                                                                                  0.0s 
 => CACHED [iot_server builder 4/7] WORKDIR /src/openssh                                                                                                                 0.0s 
 => CACHED [iot_server builder 5/7] RUN autoreconf && ./configure --prefix=/usr --sysconfdir=/etc/ssh --with-privsep-path=/var/lib/sshd --without-zlib-version-check &&  0.0s 
 => CACHED [iot_server builder 6/7] COPY start.sh /tmp/start.sh                                                                                                          0.0s 
 => CACHED [iot_server builder 7/7] RUN chmod +x /tmp/start.sh && dos2unix /tmp/start.sh                                                                                 0.0s 
 => CACHED [iot_server stage-1 3/8] COPY --from=builder /tmp/install/ /                                                                                                  0.0s 
 => CACHED [iot_server stage-1 4/8] RUN mkdir -p /var/run/sshd /var/lib/sshd && chmod 0755 /var/run/sshd                                                                 0.0s 
 => CACHED [iot_server stage-1 5/8] RUN echo "listener 1883 ::1" >> /etc/mosquitto/mosquitto.conf                                                                        0.0s
 => CACHED [iot_server stage-1 6/8] RUN echo "allow_anonymous true" >> /etc/mosquitto/mosquitto.conf                                                                     0.0s 
 => CACHED [iot_server stage-1 7/8] COPY --from=builder /tmp/start.sh /start.sh                                                                                          0.0s 
 => CACHED [iot_server stage-1 8/8] RUN chmod +x /start.sh                                                                                                               0.0s 
 => [iot_server] exporting to image                                                                                                                                      0.2s 
 => => exporting layers                                                                                                                                                  0.0s 
 => => exporting manifest sha256:4b1f0eac36a98b33fdc0b7c2a85362431546feb0e4424aad1f0d40102c792fca                                                                        0.0s 
 => => exporting config sha256:1f1022122cc2841640a93b45489ca020531c33d02ab024a5187f2297f3284ce9                                                                          0.0s 
 => => exporting attestation manifest sha256:9945a1b764930ab01e46eeba6e4cae63d653d91855cb9410f9812239f1f649e5                                                            0.1s 
 => => exporting manifest list sha256:370f886164f67726f38f5a2a51cc4bf89a860e359144431e8c4b28788f93477f                                                                   0.0s 
 => => naming to docker.io/library/endpoints-iot_server:latest                                                                                                           0.0s 
 => => unpacking to docker.io/library/endpoints-iot_server:latest                                                                                                        0.0s 
 => [iot_server] resolving provenance for metadata file                                                                                                                  0.0s 
 => [iot_device_1 internal] load build definition from Dockerfile                                                                                                        0.0s 
 => => transferring dockerfile: 318B                                                                                                                                     0.0s 
 => [iot_device_2 internal] load build definition from Dockerfile                                                                                                        0.0s 
 => => transferring dockerfile: 318B                                                                                                                                     0.0s 
 => [iot_device_1 internal] load .dockerignore                                                                                                                           0.1s 
 => => transferring context: 2B                                                                                                                                          0.0s 
 => [iot_device_2 internal] load .dockerignore                                                                                                                           0.1s 
 => => transferring context: 2B                                                                                                                                          0.0s 
 => [iot_device_1 internal] load build context                                                                                                                           0.1s 
 => => transferring context: 1.74kB                                                                                                                                      0.0s 
 => [iot_device_2 internal] load build context                                                                                                                           0.1s 
 => => transferring context: 30B                                                                                                                                         0.0s 
 => CACHED [iot_device_1 2/5] RUN apt update && apt install -y --no-install-recommends openssh-client mosquitto-clients dos2unix iproute2 && rm -rf /var/lib/apt/lists/  0.0s 
 => CACHED [iot_device_2 3/5] COPY start.sh /start.sh                                                                                                                    0.0s 
 => CACHED [iot_device_2 4/5] RUN chmod +x /start.sh                                                                                                                     0.0s 
 => CACHED [iot_device_2 5/5] RUN dos2unix /start.sh && chmod +x /start.sh                                                                                               0.0s 
 => CACHED [iot_device_1 3/5] COPY start.sh /start.sh                                                                                                                    0.0s 
 => CACHED [iot_device_1 4/5] RUN chmod +x /start.sh                                                                                                                     0.0s 
 => CACHED [iot_device_1 5/5] RUN dos2unix /start.sh && chmod +x /start.sh                                                                                               0.0s 
 => [iot_device_2] exporting to image                                                                                                                                    0.4s 
 => => exporting layers                                                                                                                                                  0.0s 
 => => exporting manifest sha256:73c01620ff8806b0113d3436790e6f4fa88761c3c0e25aad851b89550f3d8885                                                                        0.0s 
 => => exporting config sha256:1018ff6b5940bc1e0aaedcd3bdf8c5bc8442ca83bc922fe70b59060c134ba8b8                                                                          0.0s 
 => => exporting attestation manifest sha256:1442e2a39b2e6813e87f23a38c55a789b7b95b61fe1d7f17943916d88fe90b66                                                            0.3s 
 => => exporting manifest list sha256:e569467ac338909c3145faa35c6c7e9efdbc5952487bda4934fd4f409e06e5fe                                                                   0.0s 
 => => naming to docker.io/library/endpoints-iot_device_2:latest                                                                                                         0.0s 
 => => unpacking to docker.io/library/endpoints-iot_device_2:latest                                                                                                      0.0s 
 => [iot_device_1] exporting to image                                                                                                                                    0.4s 
 => => exporting layers                                                                                                                                                  0.0s 
 => => exporting manifest sha256:661df48d25693dd0e9ce348b4bd299b1699063e56ccb4147d38c5c24e0977acc                                                                        0.0s 
 => => exporting config sha256:89d18aa0fd6f2468e2140d4fe9b8bb3f86120fa6f799846e843a39bd1a472888                                                                          0.0s 
 => => exporting attestation manifest sha256:8a9a7e1879cbd93ed30ed63414f71a905c6d855227e5a231691833a93fec72c3                                                            0.2s 
 => => exporting manifest list sha256:c41f665b292a2132bb5a2f8122c773ab43e7494ea9fd3882968984fe6e67bcdf                                                                   0.0s 
 => => naming to docker.io/library/endpoints-iot_device_1:latest                                                                                                         0.0s 
 => => unpacking to docker.io/library/endpoints-iot_device_1:latest                                                                                                      0.0s 
 => [iot_device_2] resolving provenance for metadata file                                                                                                                0.1s 
 => [iot_device_1] resolving provenance for metadata file                                                                                                                0.0s
[+] Running 7/7
 ✔ iot_device_1Built0.0s 
 ✔ iot_device_2Built0.0s 
 ✔ iot_serverBuilt0.0s 
 ✔ Container iot_server    Created3.5s 
 ✔ Container bind9Created3.5s 
 ✔ Container iot_device_2  Created0.3s 
 ✔ Container iot_device_1  Created0.3s 
Attaching to bind9, iot_device_1, iot_device_2, iot_server
```

### BIND9 DNS Runtime

```bash
bind9         | Starting named...
bind9         | exec /usr/sbin/named -u "root" -g ""
bind9         | 12-Aug-2025 13:26:36.711 starting BIND 9.18.30-0ubuntu0.22.04.2-Ubuntu (Extended Support Version) <id:>
bind9         | 12-Aug-2025 13:26:36.721 running on Linux x86_64 5.15.146.1-microsoft-standard-WSL2+ #1 SMP Wed Apr 23 16:39:49 IST 2025
bind9         | 12-Aug-2025 13:26:36.721 built with  '--build=x86_64-linux-gnu' '--prefix=/usr' '--includedir=${prefix}/include' '--mandir=${prefix}/share/man' '--infodir=${prefix}/share/info' '--sysconfdir=/etc' '--localstatedir=/var' '--disable-option-checking' '--disable-silent-rules' '--libdir=${prefix}/lib/x86_64-linux-gnu' '--runstatedir=/run' '--disable-maintainer-mode' '--disable-dependency-tracking' '--libdir=/usr/lib/x86_64-linux-gnu' '--sysconfdir=/etc/bind' '--with-python=python3' '--localstatedir=/' '--enable-threads' '--enable-largefile' '--with-libtool' '--enable-shared' '--disable-static' '--with-gost=no' '--with-openssl=/usr' '--with-gssapi=yes' '--with-libidn2' '--with-json-c' '--with-lmdb=/usr' '--with-gnu-ld' '--with-maxminddb' '--with-atf=no' '--enable-ipv6' '--enable-rrl' '--enable-filter-aaaa' '--disable-native-pkcs11' 'build_alias=x86_64-linux-gnu' 'CFLAGS=-g -O2 -ffile-prefix-map=/build/bind9-AB1uwX/bind9-9.18.30=. -flto=auto -ffat-lto-objects -flto=auto -ffat-lto-objects -fstack-protector-strong -Wformat -Werror=format-security -fno-strict-aliasing -fno-delete-null-pointer-checks -DNO_VERSION_DATE -DDIG_SIGCHASE' 'LDFLAGS=-Wl,-Bsymbolic-functions -flto=auto -ffat-lto-objects -flto=auto -Wl,-z,relro -Wl,-z,now' 'CPPFLAGS=-Wdate-time -D_FORTIFY_SOURCE=2'
bind9         | 12-Aug-2025 13:26:36.721 running as: named -u root -g
bind9         | 12-Aug-2025 13:26:36.721 compiled by GCC 11.4.0
bind9         | 12-Aug-2025 13:26:36.721 compiled with OpenSSL version: OpenSSL 3.0.2 15 Mar 2022                                                                             
bind9         | 12-Aug-2025 13:26:36.721 linked to OpenSSL version: OpenSSL 3.0.2 15 Mar 2022
bind9         | 12-Aug-2025 13:26:36.721 compiled with libuv version: 1.43.0                                                                                                  
bind9         | 12-Aug-2025 13:26:36.721 linked to libuv version: 1.43.0                                                                                                      
bind9         | 12-Aug-2025 13:26:36.721 compiled with libxml2 version: 2.9.13
bind9         | 12-Aug-2025 13:26:36.721 linked to libxml2 version: 20913                                                                                                     
bind9         | 12-Aug-2025 13:26:36.721 compiled with json-c version: 0.15
bind9         | 12-Aug-2025 13:26:36.721 linked to json-c version: 0.15                                                                                                       
bind9         | 12-Aug-2025 13:26:36.721 compiled with zlib version: 1.2.11
bind9         | 12-Aug-2025 13:26:36.721 linked to zlib version: 1.2.11                                                                                                       
bind9         | 12-Aug-2025 13:26:36.721 ----------------------------------------------------
bind9         | 12-Aug-2025 13:26:36.721 BIND 9 is maintained by Internet Systems Consortium,                                                                                 
bind9         | 12-Aug-2025 13:26:36.721 Inc. (ISC), a non-profit 501(c)(3) public-benefit                                                                                    
bind9         | 12-Aug-2025 13:26:36.721 corporation.  Support and training for BIND 9 are 
bind9         | 12-Aug-2025 13:26:36.721 available at https://www.isc.org/support                                                                                             
bind9         | 12-Aug-2025 13:26:36.721 ----------------------------------------------------                                                                                 
bind9         | 12-Aug-2025 13:26:36.721 found 12 CPUs, using 12 worker threads                                                                                               
bind9         | 12-Aug-2025 13:26:36.721 using 12 UDP listeners per interface
bind9         | 12-Aug-2025 13:26:36.761 DNSSEC algorithms: RSASHA1 NSEC3RSASHA1 RSASHA256 RSASHA512 ECDSAP256SHA256 ECDSAP384SHA384 ED25519 ED448                            
bind9         | 12-Aug-2025 13:26:36.761 DS algorithms: SHA-1 SHA-256 SHA-384
bind9         | 12-Aug-2025 13:26:36.761 HMAC algorithms: HMAC-MD5 HMAC-SHA1 HMAC-SHA224 HMAC-SHA256 HMAC-SHA384 HMAC-SHA512                                                  
bind9         | 12-Aug-2025 13:26:36.761 TKEY mode 2 support (Diffie-Hellman): yes
bind9         | 12-Aug-2025 13:26:36.761 TKEY mode 3 support (GSS-API): yes                                                                                                   
bind9         | 12-Aug-2025 13:26:36.781 the initial working directory is '/'
bind9         | 12-Aug-2025 13:26:36.781 loading configuration from '/etc/bind/named.conf'                                                                                    
bind9         | 12-Aug-2025 13:26:36.781 reading built-in trust anchors from file '/etc/bind/bind.keys'
bind9         | 12-Aug-2025 13:26:36.781 looking for GeoIP2 databases in '/usr/share/GeoIP'                                                                                   
bind9         | 12-Aug-2025 13:26:36.781 using default UDP/IPv4 port range: [32768, 60999]                                                                                    
bind9         | 12-Aug-2025 13:26:36.781 using default UDP/IPv6 port range: [32768, 60999]
bind9         | 12-Aug-2025 13:26:36.781 listening on IPv4 interface lo, 127.0.0.1#53                                                                                         
bind9         | 12-Aug-2025 13:26:36.781 listening on IPv4 interface eth0, 192.168.1.29#53                                                                                    
bind9         | 12-Aug-2025 13:26:36.791 IPv6 socket API is incomplete; explicitly binding to each IPv6 address separately
bind9         | 12-Aug-2025 13:26:36.791 listening on IPv6 interface lo, ::1#53                                                                                               
bind9         | 12-Aug-2025 13:26:36.791 listening on IPv6 interface eth0, fd00::4#53
bind9         | 12-Aug-2025 13:26:36.791 listening on IPv6 interface eth0, fe80::42:c0ff:fea8:11d%309#53                                                                      
bind9         | 12-Aug-2025 13:26:36.791 Could not open '//run/named/named.pid'.                                                                                              
bind9         | 12-Aug-2025 13:26:36.791 Please check file and directory permissions or reconfigure the filename.
bind9         | 12-Aug-2025 13:26:36.791 could not open file '//run/named/named.pid': Permission denied                                                                       
bind9         | 12-Aug-2025 13:26:36.791 generating session key for dynamic DNS                                                                                               
bind9         | 12-Aug-2025 13:26:36.791 Could not open '//run/named/session.key'.
bind9         | 12-Aug-2025 13:26:36.791 Please check file and directory permissions or reconfigure the filename.                                                             
bind9         | 12-Aug-2025 13:26:36.791 could not open file '//run/named/session.key': Permission denied                                                                     
bind9         | 12-Aug-2025 13:26:36.791 could not create //run/named/session.key                                                                                             
bind9         | 12-Aug-2025 13:26:36.791 failed to generate session key for dynamic DNS: permission denied
bind9         | 12-Aug-2025 13:26:36.791 sizing zone task pool based on 1 zones
bind9         | 12-Aug-2025 13:26:36.801 zone 'myservices.local' allows unsigned updates from remote hosts, which is insecure                                                 
bind9         | 12-Aug-2025 13:26:36.801 none:99: 'max-cache-size 90%' - setting to 7112MB (out of 7903MB)                                                                    
bind9         | 12-Aug-2025 13:26:36.801 obtaining root key for view _default from '/etc/bind/bind.keys'
bind9         | 12-Aug-2025 13:26:36.801 set up managed keys zone for view _default, file 'managed-keys.bind'                                                                 
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 10.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 16.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 17.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 18.172.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 19.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 20.172.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 21.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 22.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 23.172.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 24.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 25.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 26.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 27.172.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 28.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 29.172.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 30.172.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 31.172.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 168.192.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 64.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 65.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 66.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 67.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 68.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 69.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 70.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 71.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 72.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 73.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 74.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 75.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 76.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 77.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 78.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 79.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 80.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 81.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 82.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 83.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 84.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 85.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 86.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 87.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 88.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 89.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 90.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 91.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 92.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 93.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 94.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 95.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 96.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 97.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 98.100.IN-ADDR.ARPA                                                                                            
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 99.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 100.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 101.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 102.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 103.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 104.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 105.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 106.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 107.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 108.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 109.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 110.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 111.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 112.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 113.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 114.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 115.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 116.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 117.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 118.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 119.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 120.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 121.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 122.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 123.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 124.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 125.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 126.100.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 127.100.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 0.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 127.IN-ADDR.ARPA                                                                                               
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 254.169.IN-ADDR.ARPA                                                                                           
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 2.0.192.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 100.51.198.IN-ADDR.ARPA                                                                                        
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 113.0.203.IN-ADDR.ARPA                                                                                         
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 255.255.255.255.IN-ADDR.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.IP6.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.IP6.ARPA                                       
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: D.F.IP6.ARPA                                                                                                   
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 8.E.F.IP6.ARPA                                                                                                 
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 9.E.F.IP6.ARPA                                                                                                 
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: A.E.F.IP6.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: B.E.F.IP6.ARPA                                                                                                 
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: 8.B.D.0.1.0.0.2.IP6.ARPA                                                                                       
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: EMPTY.AS112.ARPA
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: HOME.ARPA                                                                                                      
bind9         | 12-Aug-2025 13:26:36.801 automatic empty zone: RESOLVER.ARPA                                                                                                  
bind9         | 12-Aug-2025 13:26:36.811 configuring command channel from '/etc/bind/rndc.key'                                                                                
bind9         | 12-Aug-2025 13:26:36.811 open: /etc/bind/rndc.key: permission denied
bind9         | 12-Aug-2025 13:26:36.811 command channel listening on 127.0.0.1#953                                                                                           
bind9         | 12-Aug-2025 13:26:36.811 configuring command channel from '/etc/bind/rndc.key'                                                                                
bind9         | 12-Aug-2025 13:26:36.811 open: /etc/bind/rndc.key: permission denied
bind9         | 12-Aug-2025 13:26:36.871 command channel listening on ::1#953                                                                                                 
bind9         | 12-Aug-2025 13:26:36.871 not using config file logging statement for logging due to -g option                                                                 
bind9         | 12-Aug-2025 13:26:36.881 managed-keys-zone: loaded serial 0
bind9         | 12-Aug-2025 13:26:36.901 zone myservices.local/IN: loaded serial 2025041704                                                                                   
bind9         | 12-Aug-2025 13:26:36.901 all zones loaded
bind9         | 12-Aug-2025 13:26:36.911 running                                                                                                                              
bind9         | 12-Aug-2025 13:26:36.981 managed-keys-zone: Initializing automatic trust anchor management for zone '.'; DNSKEY ID 20326 is now trusted, waiving the normal 30-day waiting period.
bind9         | 12-Aug-2025 13:26:36.981 managed-keys-zone: Initializing automatic trust anchor management for zone '.'; DNSKEY ID 38696 is now trusted, waiving the normal 30-day waiting period.

```

### IoT Server and IoT Device Runtime

```bash
iot_server    | useradd: warning: the home directory /home/a23ff868ec8f already exists.
iot_server    | useradd: Not copying any file from skel directory into it.
iot_server    | Starting MQTT broker...                                                                                                                                       
iot_server    | CMX Agent a23ff868ec8f Running...
iot_device_1  | useradd: warning: the home directory /home/025f9a3db712 already exists.
iot_device_1  | useradd: Not copying any file from skel directory into it.
iot_device_2  | useradd: warning: the home directory /home/0242ac110002 already exists.                                                                                       
iot_device_2  | useradd: Not copying any file from skel directory into it.
iot_device_1  | CMX Agent 025f9a3db712 Running...                                                                                                                             
iot_device_2  | CMX Agent 0242ac110002 Running...
iot_server    | Starting MQTT subscriber...
bind9         | 12-Aug-2025 13:27:27.156 client @0x7fb88c006a98 192.168.1.3#33464: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' A 192.168.1.240
bind9         | 12-Aug-2025 13:27:27.156 client @0x7fb88c006a98 192.168.1.3#33464: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"
bind9         | 12-Aug-2025 13:27:27.156 client @0x7fb88c006a98 192.168.1.3#33464: updating zone 'myservices.local/IN': adding an RR at 'a-cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"                                                                             
iot_server    | Attempting to connect via SSH to cmxsafe-gw.myservices.local...
iot_server    | Pseudo-terminal will not be allocated because stdin is not a terminal.
iot_server    | createservice.sh was invoked at Tue Aug 12 13:27:46 UTC 2025
bind9         | 12-Aug-2025 13:28:28.152 client @0x7fb868009768 192.168.1.3#41928: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' A 192.168.1.240
bind9         | 12-Aug-2025 13:28:28.152 client @0x7fb868009768 192.168.1.3#41928: updating zone 'myservices.local/IN': adding an RR at 'dynamic-service-a23ff868ec8f.myservices.local' A 192.168.1.241 
bind9         | 12-Aug-2025 13:28:28.152 client @0x7fb868009768 192.168.1.3#41928: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"
bind9         | 12-Aug-2025 13:28:28.152 client @0x7fb868009768 192.168.1.3#41928: updating zone 'myservices.local/IN': adding an RR at 'a-cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"                                                                             
bind9         | 12-Aug-2025 13:28:28.152 client @0x7fb868009768 192.168.1.3#41928: updating zone 'myservices.local/IN': adding an RR at 'dynamic-service-a23ff868ec8f.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/dynamic-service-a23ff868ec8f"                                           
bind9         | 12-Aug-2025 13:28:28.152 client @0x7fb868009768 192.168.1.3#41928: updating zone 'myservices.local/IN': adding an RR at 'a-dynamic-service-a23ff868ec8f.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/dynamic-service-a23ff868ec8f"                                         
iot_device_1  | Attempting to connect via SSH to dynamic-service-a23ff868ec8f.myservices.local...
iot_device_2  | Attempting to connect via SSH to cmxsafe-gw.myservices.local and then a23ff868ec8f.default.svc.cluster.local...
bind9         | 12-Aug-2025 13:29:28.148 client @0x7fb88c0063a8 192.168.1.3#32812: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' A 192.168.1.240
bind9         | 12-Aug-2025 13:29:28.148 client @0x7fb88c0063a8 192.168.1.3#32812: updating zone 'myservices.local/IN': adding an RR at 'dynamic-service-a23ff868ec8f.myservices.local' A 192.168.1.241
bind9         | 12-Aug-2025 13:29:28.148 client @0x7fb88c0063a8 192.168.1.3#32812: updating zone 'myservices.local/IN': adding an RR at 'cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"                                                                               
bind9         | 12-Aug-2025 13:29:28.148 client @0x7fb88c0063a8 192.168.1.3#32812: updating zone 'myservices.local/IN': adding an RR at 'a-cmxsafe-gw.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/cmxsafe-gw"                                                                             
bind9         | 12-Aug-2025 13:29:28.148 client @0x7fb88c0063a8 192.168.1.3#32812: updating zone 'myservices.local/IN': adding an RR at 'dynamic-service-a23ff868ec8f.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/dynamic-service-a23ff868ec8f"                                           
bind9         | 12-Aug-2025 13:29:28.148 client @0x7fb88c0063a8 192.168.1.3#32812: updating zone 'myservices.local/IN': adding an RR at 'a-dynamic-service-a23ff868ec8f.myservices.local' TXT "heritage=external-dns,external-dns/owner=default,external-dns/resource=service/default/dynamic-service-a23ff868ec8f"                                         
iot_server    | Hello from IoT Device 1
iot_server    | Hello from IoT Device 2
iot_server    | Hello from IoT Device 1                                                                                                                                       
iot_server    | Hello from IoT Device 2
```

### Suspension

```bash
Gracefully stopping... (press Ctrl+C again to force)
[+] Stopping 4/4
 ✔ Container iot_device_2  Stopped                                                                                                                                      10.6s 
 ✔ Container bind9         Stopped                                                                                                                                       0.5s 
 ✔ Container iot_device_1  Stopped                                                                                                                                      10.8s 
 ✔ Container iot_server    Stopped                                                                                                                                      10.4s 
canceled
```

### Clean Up

```bash
Stopping Docker containers...
time="2025-08-12T14:45:00+01:00" level=warning msg="C:\\Users\\joann\\CMXSafeProject\\endpoints\\docker-compose.yml: the attribute `version` is obsolete, it will be ignored, please remove it to avoid potential confusion"
[+] Running 4/4
 ✔ Container iot_device_2  Removed                                                                                                                                       0.1s 
 ✔ Container iot_device_1  Removed                                                                                                                                       0.1s 
 ✔ Container bind9         Removed                                                                                                                                       0.1s 
 ✔ Container iot_server    Removed                                                                                                                                       0.1s 
release "tetragon" uninstalled
secret "cmxsafe-sshd-config" deleted
secret "cmxsafe-host-keys" deleted
Deleting Kind cluster...
Deleting cluster "cmxsafe" ...
Deleted nodes: ["cmxsafe-worker" "cmxsafe-control-plane"]
iot_network
File not found: C:\Users\joann\CMXSafeProject\endpoints\keys\iot_server\ssh\known_hosts.old
File not found: C:\Users\joann\CMXSafeProject\endpoints\keys\iot_device_1\ssh\known_hosts.old
File not found: C:\Users\joann\CMXSafeProject\endpoints\keys\iot_device_2\ssh\known_hosts.old
Cleanup completed successfully!
```

## Acknowledgements

This work was completed by Joanne Reilly and Dr. Jorge David de Hoz Diego under the supervision of Dr. Anca Delia Jurcut at the School of Computer Science, University College Dublin, Belfield, Dublin 4, Ireland.

