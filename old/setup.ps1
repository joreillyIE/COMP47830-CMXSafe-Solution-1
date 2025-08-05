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

# Function to create and deploy the cluster
function Setup-Cluster {
    
    Write-Output "Creating Network iot_network..."
    docker network create --driver=bridge --ipv6 --subnet=192.168.1.0/24 --gateway=192.168.1.1 --subnet=fd00::/64 --gateway=fd00::1 iot_network

    Write-Output "Creating Kubernetes cluster with Kind..."
    kind create cluster --name cmxsafe --config "$projectRoot\kind-config.yaml"
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to create Kind cluster"; exit $LASTEXITCODE }

    docker network connect iot_network cmxsafe-control-plane
    docker network connect iot_network cmxsafe-worker
    
    kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.14.5/config/manifests/metallb-native.yaml
    Write-Output "Creating MetalLB..."
    Start-Sleep -Seconds 90
    kubectl apply -f "$projectRoot\metallb.yaml"

    Write-Output "Creating configmap for SSH keys..."
    kubectl create configmap ssh-keys --from-file="$projectRoot\iot_project\keys"
    kubectl create configmap scripts --from-file="$projectRoot\iot_project\scripts"
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to create configmap"; exit $LASTEXITCODE }

    Write-Output "Applying Kubernetes configuration for gateway..."
    kubectl apply -f "$projectRoot\cmxsafe-gw.yaml"
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to apply cmxsafe-gw.yaml"; exit $LASTEXITCODE }

    Write-Output "Applying Kubernetes service for gateway..."
    kubectl apply -f "$projectRoot\cmxsafe-gw-service.yaml"
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to apply cmxsafe-gw-service.yaml"; exit $LASTEXITCODE }

    Write-Output "Building and starting Docker containers..."
    docker-compose -f "$projectRoot\iot_project\docker-compose.yml" up --build
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to run docker-compose"; exit $LASTEXITCODE }

    Write-Output "Setup completed successfully!"
}

# Function to remove everything created
function Cleanup-Cluster {
    Write-Output "Stopping Docker containers..."
    docker-compose -f "$projectRoot\iot_project\docker-compose.yml" down
    if ($LASTEXITCODE -ne 0) { Write-Warning "Docker-compose down failed, continuing..." }

    kubectl delete configmap ssh-keys --ignore-not-found
    kubectl delete configmap scripts --ignore-not-found
    if ($LASTEXITCODE -ne 0) { Write-Warning "Failed to delete some Kubernetes resources, continuing..." }

    Write-Output "Deleting Kind cluster..."
    kind delete cluster --name cmxsafe
    if ($LASTEXITCODE -ne 0) { Write-Warning "Kind cluster deletion failed, continuing..." }

    docker network rm iot_network

    # Delete specific files
    $filesToDelete = @(
        "$projectRoot\iot_project\keys\iot-device\known_hosts",
        "$projectRoot\iot_project\keys\iot-device\known_hosts.old",
        "$projectRoot\iot_project\keys\iot-server\known_hosts",
        "$projectRoot\iot_project\keys\iot-server\known_hosts.old"
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
