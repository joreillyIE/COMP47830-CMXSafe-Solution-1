# Ensure script stops on errors
$ErrorActionPreference = "Stop"

# Manually check if cleanup argument is passed
$Cleanup = $false
if ($args -contains "-Cleanup") {
    $Cleanup = $true
}

# Check CMXSafeProject env var is set
if (-not $Env:CMXSafeProject) {
    Write-Error "Environment variable CMXSafeProject is not set. Please set it to your project root path."
    exit 1
}

$projectRoot = $Env:CMXSafeProject

# Function to create and deploy the cluster and endpoints
function Setup-Cluster {
    
    Write-Output "Creating Network iot_network..."
    docker network create --driver=bridge --ipv6 --subnet=192.168.1.0/24 --gateway=192.168.1.1 --subnet=fd00::/64 --gateway=fd00::1 iot_network

    Write-Output "Creating Kubernetes cluster with KinD..."
    kind create cluster --name cmxsafe --config "$projectRoot\cluster\kind-config.yaml"
    if ($LASTEXITCODE -ne 0) { Write-Error "KinD create failed"; exit $LASTEXITCODE }
    kubectl wait --for=condition=Ready nodes --all
    docker network connect iot_network cmxsafe-control-plane
    docker network connect iot_network cmxsafe-worker

    Write-Output "Setting up Tetragon..."
    docker exec -it cmxsafe-control-plane sh -c 'mkdir -p /var/run/tetragon'
    docker exec -it cmxsafe-worker sh -c 'mkdir -p /var/run/tetragon'
    helm install tetragon --set tetragon.hostProcPath=/procHost cilium/tetragon -n kube-system --set exportDirectory="/var/run/tetragon" --set tetragon.grpc.enabled=true --set tetragon.grpc.address="unix:///var/run/tetragon/tetragon.sock" --set tetragon.enableProcessCred=true --set tetragon.enableProcessNs=true --set tetragonOperator.PodInfo.enabled=true
    kubectl rollout status -n kube-system ds/tetragon -w

    Write-Output "Setting up K8S Go Client..."
    docker build -t myclient:dev "$projectRoot\cluster\k8s-client"
    if ($LASTEXITCODE -ne 0) { Write-Error "K8S Go Client build failed"; exit $LASTEXITCODE }
    kind --name cmxsafe load docker-image myclient:dev
    if ($LASTEXITCODE -ne 0) { Write-Error "K8S Go Client image load (myclient) failed"; exit $LASTEXITCODE }
    kubectl apply -f "$projectRoot\cluster\k8s-client\client.yaml"

    Write-Output "Setting up MetalLB..."
    kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.14.5/config/manifests/metallb-native.yaml
    if ($LASTEXITCODE -ne 0) { Write-Error "MetalLB install failed"; exit $LASTEXITCODE }
    kubectl wait --for=condition=Established crd/ipaddresspools.metallb.io crd/l2advertisements.metallb.io
    kubectl -n metallb-system rollout status deploy/controller
    kubectl -n metallb-system rollout status ds/speaker
    kubectl apply -f "$projectRoot\cluster\metal-lb\metallb.yaml"

    Write-Output "Setting up External DNS..."
    kubectl apply -f "$projectRoot\cluster\external-dns\external-dns.yaml"

    Write-Output "Creating Secrets for SSH Host Keys and Configurations..."
    kubectl create secret generic cmxsafe-sshd-config --from-file="$projectRoot\cluster\cmxsafe-gw\config"
    kubectl create secret generic cmxsafe-host-keys --from-file="$projectRoot\cluster\cmxsafe-gw\keys"
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to create secrets"; exit $LASTEXITCODE }

    Write-Output "Creating Secrets for IoT Users and Authorized Keys..."
    kubectl apply -f "$projectRoot\cluster\cmxsafe-gw\iot-users.yaml"
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to apply iot-users.yaml"; exit $LASTEXITCODE }

    Write-Output "Applying Kubernetes configuration for gateways..."
    docker build -t gateway:dev "$projectRoot\cluster\cmxsafe-gw"
    kind --name cmxsafe load docker-image gateway:dev
    kubectl apply -f "$projectRoot\cluster\cmxsafe-gw\cmxsafe-gw.yaml"
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to apply cmxsafe-gw.yaml"; exit $LASTEXITCODE }

    Write-Output "Applying Kubernetes service for gateways..."
    kubectl apply -f "$projectRoot\cluster\cmxsafe-gw\cmxsafe-gw-service.yaml"
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to apply cmxsafe-gw-service.yaml"; exit $LASTEXITCODE }

    Write-Output "Building and starting Docker containers..."
    docker-compose -f "$projectRoot\endpoints\docker-compose.yml" up --build
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to run docker-compose"; exit $LASTEXITCODE }

    Write-Output "Setup completed successfully!"
}

# Function to remove everything created
function Cleanup-Cluster {
    Write-Output "Stopping Docker containers..."
    docker-compose -f "$projectRoot\endpoints\docker-compose.yml" down
    if ($LASTEXITCODE -ne 0) { Write-Warning "Docker-compose down failed, continuing..." }
    
    helm uninstall tetragon -n kube-system
    kubectl wait --for=delete ds/tetragon -n kube-system
    kubectl delete secret cmxsafe-sshd-config --ignore-not-found
    kubectl delete secret cmxsafe-host-keys --ignore-not-found
    if ($LASTEXITCODE -ne 0) { Write-Warning "Failed to delete some Kubernetes resources, continuing..." }

    Write-Output "Deleting Kind cluster..."
    kind delete cluster --name cmxsafe
    if ($LASTEXITCODE -ne 0) { Write-Warning "Kind cluster deletion failed, continuing..." }

    docker network rm iot_network

    # Delete specific files
    $filesToDelete = @(
        "$projectRoot\endpoints\keys\iot_server\ssh\known_hosts.old",
        "$projectRoot\endpoints\keys\iot_device_1\ssh\known_hosts.old",
        "$projectRoot\endpoints\keys\iot_device_2\ssh\known_hosts.old"
    )

    foreach ($file in $filesToDelete) {
        if (Test-Path $file) {
            Write-Output "Deleting file: $file"
            Remove-Item -Path $file -Force -ErrorAction SilentlyContinue
            if (Test-Path $file) {
                Write-Warning "Failed to delete: $file"
            } else {
                Write-Output "Successfully deleted: $file"
            }
        } else {
            Write-Output "File not found: $file"
        }
    }

    Write-Output "Cleanup completed successfully!"
}

# Run the appropriate function based on the argument
if ($Cleanup) {
    Cleanup-Cluster
} else {
    Setup-Cluster
}