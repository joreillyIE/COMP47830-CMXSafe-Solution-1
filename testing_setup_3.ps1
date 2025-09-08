# Ensure script stops on errors
$ErrorActionPreference = "Stop"

# Manually check if cleanup argument is passed
$Cleanup = $false
if ($args -contains "-Cleanup") {
    $Cleanup = $true
}

$Deploy = "cmxsafe-gw"
$replicas = $args[0] -as [int]
if ($replicas -in 1, 10, 20, 30, 40, 50) {
    $Deploy = "$Deploy-$replicas"   # e.g., "cmxsafe-gw-10"
} else {
    $replicas = 3
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
    helm repo add cilium https://helm.cilium.io
    helm repo update
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

    Write-Output "Applying Kubernetes service for gateways..."
    kubectl apply -f "$projectRoot\cluster\cmxsafe-gw\cmxsafe-gw-service.yaml"
    if ($LASTEXITCODE -ne 0) { Write-Error "Failed to apply cmxsafe-gw-service.yaml"; exit $LASTEXITCODE }

    #Write-Output "Building and starting Docker containers..."
    #docker-compose -f "$projectRoot\endpoints\docker-compose.yml" up --build
    #if ($LASTEXITCODE -ne 0) { Write-Error "Failed to run docker-compose"; exit $LASTEXITCODE }

    Write-Output "Setup completed successfully! Waiting before setting up tests..."

    # Output File
    $logPath = Join-Path $PWD ("/test/cmxsafe-gw_rollout_{0}.txt" -f (Get-Date -Format 'yyyyMMdd_HHmmss'))
    $lines = @()


    # ============================================================== BEGIN: Deployment Tests ==============================================================
    Write-Output "Applying Kubernetes Deployment for CMX-GWs..."
    # Deploy replica set of X pods
    $deploymentCreation = Start-Job -ScriptBlock {
        kubectl get deploy -n default --watch-only --field-selector=metadata.name=cmxsafe-gw -o name | Select-Object -First 1 > $null
        Get-Date
    }
    docker build -t gateway:dev "$projectRoot\cluster\cmxsafe-gw"
    kind --name cmxsafe load docker-image gateway:dev
    kubectl apply -f "$projectRoot\cluster\cmxsafe-gw\test-cases\$Deploy.yaml"
    $declaredAt = Get-Date
    Wait-Job -Job $deploymentCreation
    $firstPodCreation = Start-Job -ScriptBlock {
        do {
            $count = kubectl get deploy cmxsafe-gw -n default -o jsonpath='{.status.replicas}' 2>$null
        } while (($count -as [int]) -eq 0)
        Get-Date
    }
    $firstPodScheduled = Start-Job -ScriptBlock {
        do {
            $scheduled = $(kubectl get pods -n default -l app=cmxsafe-gw -o json | ConvertFrom-Json | Select-Object -ExpandProperty items | Where-Object { $_.status.conditions | Where-Object { $_.type -eq "PodScheduled" -and $_.status -eq "True" } })
        } while (-not $scheduled)
        Get-Date
    }
    $firstPodReady = Start-Job -ScriptBlock {
        do {
            $count = kubectl get deploy cmxsafe-gw -n default -o jsonpath='{.status.availableReplicas}' 2>$null
        } while (($count -as [int]) -eq 0)
        Get-Date
    }
    $allPodsCreation = Start-Job -ScriptBlock {
        $desired = $using:replicas
        $cmd = "kubectl wait deploy/cmxsafe-gw -n default --for=jsonpath='{.status.replicas}'=$desired --timeout=30m"
        Invoke-Expression $cmd | Out-Null
        Get-Date
    }
    $allPodsScheduled = Start-Job -ScriptBlock {
        do {
            $names = kubectl get pods -n default -l app=cmxsafe-gw -o name 2>$null
        } while (-not $names)
        kubectl wait -n default --for=condition=PodScheduled --selector=app=cmxsafe-gw pods --timeout=30m | Out-Null
        Get-Date
    }
    $allPodsReady = Start-Job -ScriptBlock {
        $desired = $using:replicas
        $cmd = "kubectl wait deploy/cmxsafe-gw -n default --for=jsonpath='{.status.availableReplicas}'=$desired --timeout=30m"
        Invoke-Expression $cmd | Out-Null
        Get-Date
    }
    Wait-Job -Job $firstPodCreation, $allPodsCreation, $firstPodScheduled, $allPodsScheduled, $firstPodReady, $allPodsReady

    $lines += "Deployment of $replicas CMX-GWs"
    $lines += ("Deployment Declared At: {0:o}" -f $declaredAt)
    $lines += ("Deployment Created At: {0:o}" -f ((Receive-Job -Job $deploymentCreation) | Select-Object -Last 1))
    if ($replicas -eq 1) {
        $lines += ("Pod Creation Ends At: {0:o}" -f ((Receive-Job -Job $firstPodCreation) | Select-Object -Last 1))
        $lines += ("Pod Scheduling Ends At: {0:o}" -f ((Receive-Job -Job $firstPodScheduled) | Select-Object -Last 1))
        $lines += ("Pod Readying Ends At: {0:o}" -f ((Receive-Job -Job $firstPodReady) | Select-Object -Last 1))
    } else {
        $lines += ("Pod Creation Starts At: {0:o}" -f ((Receive-Job -Job $firstPodCreation) | Select-Object -Last 1))
        $lines += ("Pod Creation Ends At: {0:o}" -f ((Receive-Job -Job $allPodsCreation) | Select-Object -Last 1))
        $lines += ("Pod Scheduling Starts At: {0:o}" -f ((Receive-Job -Job $firstPodScheduled) | Select-Object -Last 1))
        $lines += ("Pod Scheduling Ends At: {0:o}" -f ((Receive-Job -Job $allPodsScheduled) | Select-Object -Last 1))
        $lines += ("Pod Readying Starts At: {0:o}" -f ((Receive-Job -Job $firstPodReady) | Select-Object -Last 1))
        $lines += ("Pod Readying Ends At: {0:o}" -f ((Receive-Job -Job $allPodsReady) | Select-Object -Last 1))
    }
    $lines += ""
    Get-Job | Remove-Job -Force
    # ============================================================== END: Deployment Tests ==============================================================
    # ============================================================== BEGIN: Scale Up Tests ==============================================================
    Write-Output "Scaling up CMX-GWs..."
    $target = $replicas * 2
    $deploymentScaleUp = Start-Job -ScriptBlock {
        $desired = $using:target
        $cmd = "kubectl wait deploy/cmxsafe-gw -n default --for=jsonpath='{.spec.replicas}'=$desired --timeout=30m"
        Invoke-Expression $cmd | Out-Null
        Get-Date
    }
    kubectl scale deployment/cmxsafe-gw -n default --replicas=$target
    $declaredAt = Get-Date
    $firstPodCreation = Start-Job -ScriptBlock {
        do {
            $count = kubectl get deploy cmxsafe-gw -n default -o jsonpath='{.status.replicas}' 2>$null
        } while (($count -as [int]) -le $using:replicas)
        Get-Date
    }
    $firstPodScheduled = Start-Job -ScriptBlock {
        do {
            $scheduled = $(kubectl get pods -n default -l app=cmxsafe-gw -o json | ConvertFrom-Json | Select-Object -ExpandProperty items | Where-Object { $_.status.conditions | Where-Object { $_.type -eq "PodScheduled" -and $_.status -eq "True" } })
        } while ($scheduled.Count -le $using:replicas)
        Get-Date
    }
    $firstPodReady = Start-Job -ScriptBlock {
        do {
            $count = kubectl get deploy cmxsafe-gw -n default -o jsonpath='{.status.availableReplicas}' 2>$null
        } while (($count -as [int]) -le $using:replicas)
        Get-Date
    }
    $allPodsCreation = Start-Job -ScriptBlock {
        $desired = $using:target
        $cmd = "kubectl wait deploy/cmxsafe-gw -n default --for=jsonpath='{.status.replicas}'=$desired --timeout=30m"
        Invoke-Expression $cmd | Out-Null
        Get-Date
    }
    $allPodsScheduled = Start-Job -ScriptBlock {
        do {
            $names = kubectl get pods -n default -l app=cmxsafe-gw -o name 2>$null
        } while ($names.Count -le $using:replicas)
        kubectl wait -n default --for=condition=PodScheduled --selector=app=cmxsafe-gw pods --timeout=30m | Out-Null
        Get-Date
    }
    $allPodsReady = Start-Job -ScriptBlock {
        $desired = $using:target
        $cmd = "kubectl wait deploy/cmxsafe-gw -n default --for=jsonpath='{.status.availableReplicas}'=$desired --timeout=30m"
        Invoke-Expression $cmd | Out-Null
        Get-Date
    }
    Wait-Job -Job $deploymentScaleUp, $firstPodCreation, $allPodsCreation, $firstPodScheduled, $allPodsScheduled, $firstPodReady, $allPodsReady

    $lines += "Scale up from $replicas CMX-GWs to $target CMX-GWs"
    $lines += ("Scale Up Declared At: {0:o}" -f $declaredAt)
    $lines += ("Deployment Updated At: {0:o}" -f ((Receive-Job -Job $deploymentScaleUp) | Select-Object -Last 1))
    if ($replicas -eq 1) {
        $lines += ("Pod Creation Ends At: {0:o}" -f ((Receive-Job -Job $firstPodCreation) | Select-Object -Last 1))
        $lines += ("Pod Scheduling Ends At: {0:o}" -f ((Receive-Job -Job $firstPodScheduled) | Select-Object -Last 1))
        $lines += ("Pod Readying Ends At: {0:o}" -f ((Receive-Job -Job $firstPodReady) | Select-Object -Last 1))
    } else {
        $lines += ("Pod Creation Starts At: {0:o}" -f ((Receive-Job -Job $firstPodCreation) | Select-Object -Last 1))
        $lines += ("Pod Creation Ends At: {0:o}" -f ((Receive-Job -Job $allPodsCreation) | Select-Object -Last 1))
        $lines += ("Pod Scheduling Starts At: {0:o}" -f ((Receive-Job -Job $firstPodScheduled) | Select-Object -Last 1))
        $lines += ("Pod Scheduling Ends At: {0:o}" -f ((Receive-Job -Job $allPodsScheduled) | Select-Object -Last 1))
        $lines += ("Pod Readying Starts At: {0:o}" -f ((Receive-Job -Job $firstPodReady) | Select-Object -Last 1))
        $lines += ("Pod Readying Ends At: {0:o}" -f ((Receive-Job -Job $allPodsReady) | Select-Object -Last 1))
    }
    $lines += ""
    Get-Job | Remove-Job -Force
    # ============================================================== END: Scale Up Tests ==============================================================
    # ============================================================== BEGIN: Self Healing Tests ==============================================================
    Write-Output "Killing off CMX-GWs..."
    $toJudge = (kubectl get pods -n default -l app=cmxsafe-gw -o json | ConvertFrom-Json).items
    $baselineUIDs = $toJudge | ForEach-Object { $_.metadata.uid }
    $toKill = $toJudge | Get-Random -Count $replicas
    $toLive = $toJudge | Where-Object { $toKill -notcontains $_ }
    $toKillNames = $toKill | ForEach-Object { $_.metadata.name }
    $toLiveNames = $toLive | ForEach-Object { $_.metadata.name }
    foreach ($name in $toKillNames) { kubectl label pod $name -n default group=kill }
    foreach ($name in $toLiveNames) { kubectl label pod $name -n default group=live }
    # Function to record moment desired set no longer equals current set
    $deploymentSelfHeal = Start-Job -ScriptBlock {
        $desired = $using:replicas
        $cmd = "kubectl wait deploy/cmxsafe-gw -n default --for=jsonpath='{.status.availableReplicas}'=$desired --timeout=30m"
        Invoke-Expression $cmd | Out-Null
        Get-Date
    }
    # Delete pods
    kubectl delete pod -n default -l app=cmxsafe-gw,group=kill --wait=false
    $declaredAt = Get-Date
    $firstPodCreation = Start-Job -ScriptBlock {
        do {
            $count = kubectl get pods -n default -l app=cmxsafe-gw,!group -o name 2>$null
        } while (-not $count)
        Get-Date
    }
    $firstPodScheduled = Start-Job -ScriptBlock {
        do {
            $scheduled = $(kubectl get pods -n default -l app=cmxsafe-gw,!group -o json | ConvertFrom-Json | Select-Object -ExpandProperty items | Where-Object { $_.status.conditions | Where-Object { $_.type -eq "PodScheduled" -and $_.status -eq "True" } })
        } while (-not $scheduled)
        Get-Date
    }
    $firstPodReady = Start-Job -ScriptBlock {
        do {
            $count = kubectl get deploy cmxsafe-gw -n default -o jsonpath='{.status.availableReplicas}' 2>$null
        } while (($count -as [int]) -le $using:replicas)
        Get-Date
    }
    $allPodsCreation = Start-Job -ScriptBlock {
        do {
            $names = kubectl get pods -n default -l app=cmxsafe-gw,!group -o name 2>$null
        } while ($names.Count -lt $using:replicas)
        Get-Date
    }
    $allPodsScheduled = Start-Job -ScriptBlock {
        do {
            $names = kubectl get pods -n default -l app=cmxsafe-gw,!group -o name 2>$null
        } while ($names.Count -lt $using:replicas)
        kubectl wait -n default --for=condition=PodScheduled --selector=app=cmxsafe-gw,!group pods --timeout=30m | Out-Null
        Get-Date
    }
    $allPodsReady = Start-Job -ScriptBlock {
        $desired = $using:target
        $cmd = "kubectl wait deploy/cmxsafe-gw -n default --for=jsonpath='{.status.availableReplicas}'=$desired --timeout=30m"
        Invoke-Expression $cmd | Out-Null
        Get-Date
    }
    Wait-Job -Job $deploymentSelfHeal, $firstPodCreation, $allPodsCreation, $firstPodScheduled, $allPodsScheduled, $firstPodReady, $allPodsReady

    $lines += "Self heal from killing $replicas of $target CMX-GWs"
    $lines += ("Deletion Declared At: {0:o}" -f $declaredAt)
    $lines += ("Deployment Updated At: {0:o}" -f ((Receive-Job -Job $deploymentSelfHeal) | Select-Object -Last 1))
    if ($replicas -eq 1) {
        $lines += ("Pod Creation Ends At: {0:o}" -f ((Receive-Job -Job $firstPodCreation) | Select-Object -Last 1))
        $lines += ("Pod Scheduling Ends At: {0:o}" -f ((Receive-Job -Job $firstPodScheduled) | Select-Object -Last 1))
        $lines += ("Pod Readying Ends At: {0:o}" -f ((Receive-Job -Job $firstPodReady) | Select-Object -Last 1))
    } else {
        $lines += ("Pod Creation Starts At: {0:o}" -f ((Receive-Job -Job $firstPodCreation) | Select-Object -Last 1))
        $lines += ("Pod Creation Ends At: {0:o}" -f ((Receive-Job -Job $allPodsCreation) | Select-Object -Last 1))
        $lines += ("Pod Scheduling Starts At: {0:o}" -f ((Receive-Job -Job $firstPodScheduled) | Select-Object -Last 1))
        $lines += ("Pod Scheduling Ends At: {0:o}" -f ((Receive-Job -Job $allPodsScheduled) | Select-Object -Last 1))
        $lines += ("Pod Readying Starts At: {0:o}" -f ((Receive-Job -Job $firstPodReady) | Select-Object -Last 1))
        $lines += ("Pod Readying Ends At: {0:o}" -f ((Receive-Job -Job $allPodsReady) | Select-Object -Last 1))
    }
    $lines += ""
    Get-Job | Remove-Job -Force
    # ============================================================== END: Self Healing Tests ==============================================================
    # ============================================================== BEGIN: Scale Down Tests ==============================================================
    Write-Output "Scaling down CMX-GWs..."
    $deploymentScaleDown = Start-Job -ScriptBlock {
        $desired = $using:replicas
        $cmd = "kubectl wait deploy/cmxsafe-gw -n default --for=jsonpath='{.spec.replicas}'=$desired --timeout=30m"
        Invoke-Expression $cmd | Out-Null
        Get-Date
    }
    kubectl scale deployment/cmxsafe-gw -n default --replicas=$replicas
    $declaredAt = Get-Date
    $firstPodScheduledForTerm = Start-Job -ScriptBlock {
        do {
            $json = kubectl get pods -n default -l app=cmxsafe-gw -o json 2>$null
            $pods = if ($json) { $json | ConvertFrom-Json } else { $null }
            $marked = @()
            if ($pods) { $marked = $pods.items | Where-Object { $_.metadata.deletionTimestamp } }
        } while (-not $marked)
        Get-Date
    }
    $allPodsScheduledForTerm = Start-Job -ScriptBlock {
        do {
            $json  = kubectl get pods -n default -l app=cmxsafe-gw -o json 2>$null
            $items = if ($json) { @((($json | ConvertFrom-Json).items)) } else { @() }
            $marked = $items | Where-Object { $_.metadata.deletionTimestamp }
            $nonTerminating = $items.Count - $marked.Count
        } while ($nonTerminating -gt $using:replicas)
        Get-Date
    }
    $firstPodDelete = Start-Job -ScriptBlock {
        do {
            $names = kubectl get pods -n default -l app=cmxsafe-gw -o name 2>$null
        } while ($names.Count -ge $using:target)
        Get-Date
    }
    $allPodsDelete = Start-Job -ScriptBlock {
        do {
            $names = kubectl get pods -n default -l app=cmxsafe-gw -o name 2>$null
        } while ($names.Count -gt $using:replicas)
        Get-Date
    }
    Wait-Job -Job $deploymentScaleDown, $firstPodScheduledForTerm, $allPodsScheduledForTerm, $firstPodDelete, $allPodsDelete

    $lines += "Scale down from $target to $replicas CMX-GWs"
    $lines += ("Scale Down Declared At: {0:o}" -f $declaredAt)
    $lines += ("Deployment Updated At: {0:o}" -f ((Receive-Job -Job $deploymentScaleDown) | Select-Object -Last 1))
    if ($replicas -eq 1) {
        $lines += ("Pod Termination Ends At: {0:o}" -f ((Receive-Job -Job $firstPodScheduledForTerm) | Select-Object -Last 1))
        $lines += ("Pod Deletion Ends At: {0:o}" -f ((Receive-Job -Job $firstPodDelete) | Select-Object -Last 1))
    } else {
        $lines += ("Pod Termination Starts At: {0:o}" -f ((Receive-Job -Job $firstPodScheduledForTerm) | Select-Object -Last 1))
        $lines += ("Pod Termination Ends At: {0:o}" -f ((Receive-Job -Job $allPodsScheduledForTerm) | Select-Object -Last 1))
        $lines += ("Pod Deletion Starts At: {0:o}" -f ((Receive-Job -Job $firstPodDelete) | Select-Object -Last 1))
        $lines += ("Pod Deletion Ends At: {0:o}" -f ((Receive-Job -Job $allPodsDelete) | Select-Object -Last 1))
    }
    $lines += ""
    Get-Job | Remove-Job -Force
    # ============================================================== END: Scale Down Tests ==============================================================
    Set-Content -Path $logPath -Value $lines -Encoding UTF8
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