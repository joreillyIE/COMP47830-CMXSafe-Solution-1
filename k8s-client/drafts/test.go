package main

import (
	"context"
	"fmt"
	//"time"
        //"io"
	"log"
        "encoding/json"
	"sync"
	"path/filepath"
	"os"
	"strings"
	
	"k8s.io/apimachinery/pkg/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"google.golang.org/grpc"
        "google.golang.org/grpc/credentials/insecure"
        //"google.golang.org/grpc/test/bufconn"
        tetragon "github.com/cilium/tetragon/api/v1/tetragon"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/utils/pointer"
        "net"

	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

// global cache
var podIPCache sync.Map // map[string] = string (namespace/podname → IP)
var serviceCache sync.Map
// ----------- Tetragon gRPC Setup -----------

func connectTetragonGRPC() *grpc.ClientConn {
	socket := "/var/run/tetragon/tetragon.sock"
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return net.Dial("unix", socket)
	}
	conn, err := grpc.DialContext(context.Background(), "unix://"+socket,
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect to tetragon socket: %v", err)
	}
	return conn
}

// ----------- Kubernetes Setup -----------

func setupKubeClient() *kubernetes.Clientset {
	// client-go uses the Service Account token mounted inside the Pod
	config, err := rest.InClusterConfig()

	if err != nil {
		// Fall back to kubeconfig (for dev outside the cluster)
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("failed to create Kubernetes config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("clientset error: %v", err)
	}
	return clientset
}

func startPodInformer(clientset *kubernetes.Clientset, stopCh <-chan struct{}) {
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0,
	informers.WithNamespace("default"),
	informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = "app=cmxsafe-gw"
	}))
	informer := factory.Core().V1().Pods().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			p := obj.(*corev1.Pod)
			key := p.Namespace + "/" + p.Name
			podIPCache.Store(key, p.Status.PodIP)
		},
		UpdateFunc: func(_, newObj interface{}) {
			p := newObj.(*corev1.Pod)
			key := p.Namespace + "/" + p.Name
			podIPCache.Store(key, p.Status.PodIP)
		},
		DeleteFunc: func(obj interface{}) {
			p := obj.(*corev1.Pod)
			key := p.Namespace + "/" + p.Name
			podIPCache.Delete(key)
		},
	})

	factory.Start(stopCh)
	cache.WaitForCacheSync(stopCh, informer.HasSynced)
	log.Println("Pod informer started.")
}

func startServiceInformer(clientset *kubernetes.Clientset, stopCh <-chan struct{}) {
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0,
		informers.WithNamespace(""),
	)

	svcInformer := factory.Core().V1().Services().Informer()

	svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				if dns, ok := svc.Annotations["external-dns.alpha.kubernetes.io/hostname"]; ok {
        				if strings.HasSuffix(dns, ".myservices.local") {						
						key := strings.TrimSuffix(dns, ".myservices.local")
						newVal := svc.Namespace + "/" + svc.Name + ";"
						// check if key exists in map
						if storedVal, ok := serviceCache.Load(key); ok {
							if strings.Count(storedVal, newVal) < 1 {
								serviceCache.Store(key, storedVal + newVal)
							}
						} else {
							serviceCache.Store(key, newVal)
						}
        				}
    				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newSVC := newObj.(*corev1.Service)
			oldSVC := oldObj.(*corev1.Service)
			// if both are LBs & one LB has an annotation & one of the annotations has the suffix
			if oldSVC.Spec.Type == corev1.ServiceTypeLoadBalancer || oldSVC.Spec.Type == corev1.ServiceTypeLoadBalancer {
				oldDNS, bool1 := oldSVC.Annotations["external-dns.alpha.kubernetes.io/hostname"]
				newDNS, bool2 := newSVC.Annotations["external-dns.alpha.kubernetes.io/hostname"]
				if bool1 || bool2 {
				oldBool := strings.HasSuffix(oldDNS, ".myservices.local")
				newBool := strings.HasSuffix(newDNS, ".myservices.local")
					if oldBool || newBool {
						if oldBool {
							oldKey := strings.TrimSuffix(oldDNS, ".myservices.local")
							if oldVal, ok := serviceCache.Load(oldKey); ok {
								string1 := oldSVC.Namespace + "/" + oldSVC.Name + ";"
								if strings.Count(oldVal, "/") > 1 {
									string2 := ""
									if newBool && stringsMatch(oldDNS, newDNS) {
										string2 = newSVC.Namespace + "/" + newSVC.Name + ";"
									}
									serviceCache.Store(oldKey, replaceIfContains(oldVal, string1, string2))
								} else {
									if stringsMatch(oldVal, string1) {
										Cache.Delete(oldKey)	
									}
								}
							}
						}

						if newBool {
							newKey := strings.TrimSuffix(newDNS, ".myservices.local")
							newName := newSVC.Namespace + "/" + newSVC.Name + ";"
							if newVal, ok := serviceCache.Load(newKey); ok {
								if strings.Count(newVal, newName) < 1 {
									serviceCache.Store(newKey, newVal + newName)
								}
							} else {
								serviceCache.Store(newKey, newName)
							}
						}
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				if dns, ok := svc.Annotations["external-dns.alpha.kubernetes.io/hostname"]; ok {
        				if strings.HasSuffix(dns, ".myservices.local") {						
						key := strings.TrimSuffix(dns, ".myservices.local")
						oldVal := svc.Namespace + "/" + svc.Name + ";"
						// check if key exists in map
						if storedVal, ok := serviceCache.Load(key); ok {
							if strings.Count(storedVal, "/") > 1 {
								serviceCache.Store(key, replaceIfContains(storedVal, oldVal, ""))
							} else {
								if stringsMatch(storedVal, oldVal) {
									Cache.Delete(key)
								}
							}
						}
        				}
    				}
			}

		},
	})

	factory.Start(stopCh)
	cache.WaitForCacheSync(stopCh, svcInformer.HasSynced)
	log.Println("Service informer started")
}


func lookupPodIP(namespace, podName string) string {
	key := namespace + "/" + podName
	if val, ok := podIPCache.Load(key); ok {
		return val.(string)
	}
	return "(not cached)"
}

func main() {

	stopCh := make(chan struct{})
	defer close(stopCh)

        // Start Kubernetes client + informer
	clientset := setupKubeClient()
	go startPodInformer(clientset, stopCh)
	go startServiceInformer(clientset, stopCh)

        // Connect to Tetragon gRPC
	conn := connectTetragonGRPC()
	defer conn.Close()

        client := tetragon.NewFineGuidanceSensorsClient(conn)

        log.Printf("Client returned...")

        stream, err := client.GetEvents(context.Background(), &tetragon.GetEventsRequest{
  		AllowList: []*tetragon.Filter{{
      			EventSet:           []tetragon.EventType{tetragon.EventType_PROCESS_EXEC},
      			ArgumentsRegex: []string{".*createservice\\.sh.*"},
      			PodRegex:           []string{`^cmxsafe-gw-.*`},
      			BinaryRegex:  []string{`^/usr/bin/logger$`},
    		},},
	})

        if err != nil {
                fmt.Printf("GetEvents failed: %v", err)
        }

        log.Printf("Stream returned...")

        for {
		resp, err := stream.Recv()
        	if err != nil {
        		log.Fatalf("Stream closed: %v", err)
        	}
        	exec := resp.GetProcessExec()
        	if exec == nil {
        		continue
        	}
        	proc := exec.Process
		jsonBytes, err := json.MarshalIndent(proc, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal process: %v", err)
		} else {
			fmt.Println(string(jsonBytes))
		}
		podName := proc.Pod.Name
		ns := proc.Pod.Namespace
		podIP := lookupPodIP(ns, podName)
		username := extractUsername(proc.Cwd)

		fmt.Printf("Matched script exec in pod %s/%s (IP: %s)\n", ns, podName, podIP)

		if username != "" && podIP != "" { 
			prefix := "dynamic-service-"
			suffix := ".myservices.local"
			label := handleNSService(clientset, prefix, username, suffix)
			err := handleEndpoint(clientset, label, username, podIP)
			if err != nil {
				log.Printf("Failed to patch or create North-South service: %v", err)
			}
			err := handleEWService(clientset, username, podIP)
			if err != nil {
				log.Printf("Failed to patch or create East-West service: %v", err)
			}
		}
	}

}

func handleEWService(client *kubernetes.Clientset, username, podIP string) error {
	err = patchExternalService(clientset, ns, username, podIP)
	if apierrors.IsNotFound(err) {
		err = createEWService(clientset, ns, username, podIP)
	}
	return err
}

func handleEndpoint(client *kubernetes.Clientset, label, username, podIP string) error {
	// get all endpoint slices with selector and ports
	sliceClient := client.DiscoveryV1().EndpointSlices("")

	labelSelector := fmt.Sprintf("kubernetes.io/service-name=%s", label)

	slices, err := sliceClient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err != nil {
		return nil, err
	}

	var filteredSlices []discoveryv1.EndpointSlice

	for _, slice := range slices.Items {
		for _, port := range slice.Ports {
			if port.Port != nil && *port.Port == 22 && *port.Protocol == "TCP" {
				filteredSlices = append(filteredSlices, slice)
				break
			}
			if port.Port != nil && *port.Port == 2222 && *port.Protocol == "TCP" {
				filteredSlices = append(filteredSlices, slice)
				break
			}
		}
	}

	count := len(filteredSlices)
	// if > 1, remove selectors

	var filteredSlices2 []discoveryv1.EndpointSlice

	for _, slice := range filteredSlices {
		if slice.Namespace == "default" && slice.Name == username + "-slice" {
			filteredSlices2 = append(filteredSlices2, slice)
			continue
		}

		// Patch to remove the service-name label
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"kubernetes.io/service-name": nil,
				},
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			log.Printf("Failed to marshal patch for %s/%s: %v", slice.Namespace, slice.Name, err)
			filteredSlices2 = append(filteredSlices2, slice)
			continue
		}

		_, err = client.DiscoveryV1().EndpointSlices(slice.Namespace).Patch(
			context.TODO(),
			slice.Name,
			types.MergePatchType,
			patchBytes,
			metav1.PatchOptions{},
		)
		if err != nil {
			log.Printf("Failed to patch EndpointSlice %s/%s: %v", slice.Namespace, slice.Name, err)
			filteredSlices2 = append(filteredSlices2, slice)
		} else {
			log.Printf("Patched EndpointSlice %s/%s", slice.Namespace, slice.Name)
		}
	}

	count = len(filteredSlices2)

	// if 1, patch to ensure correctness
	if count == 1 {
		remainingSlice := filteredSlices2[0]
		err := patchEndpointSlices(client, remainingSlice.Namespace, remainingSlice.Name, label, podIP)
	}

	// if 0, create
	if count == 0 {
		err := createEndpointSlice(client, label, username, podIP)
	}
	return err
}

func handleNSService(client *kubernetes.Clientset, prefix, username, suffix string) string {
	// Get list of services with annotation prefix + username
	key := prefix + username
	list := ""
	count := 0
	if val, ok := serviceCache.Load(key); ok {
		count = strings.Count(val, ";")
		list = val
	}

	if count > 1 {
		replaceIfContains(storedVal, oldVal, "")
		entries := strings.Split(replaceIfContains(list, "default/" + prefix + username + ";", ""), ";")
		for _, entry := range entries {
			if entry == "" {
				continue
			}
	
			parts := strings.SplitN(entry, "/", 2)
			if len(parts) != 2 {
				continue // or log an error...
			}
			namespace := parts[0]
			name := parts[1]
	
			err := removeNSServiceAnnotation(client, namespace, name)
			if err == nil {
				list = replaceIfContains(list, namespace + "/" + prefix + username + ";", "")
			}
		}
	}

	count = strings.Count(list, ";")

	if count == 1 {
		parts := strings.SplitN(list, "/", 2)
		namespace := parts[0]
		name := parts[1]

		err := patchNSService(client, namespace, name, prefix, username, suffix)
		if err == nil {
			return name
		}
	}

	if count == 0 {
		err := createNSService(client, prefix, username, suffix)
		if err == nil {
			return prefix + username
		}
	}

	log.Printf("Failed to patch or create North-South service: %v", err)
	return ""
}

func extractUsername(cwd string) string {
	parts := strings.Split(cwd, "/")
	if len(parts) >= 3 && parts[1] == "home" {
		return parts[2]
	}
	return ""
}

func ipDSH(ip string) string {
	return strings.ReplaceAll(ip, ".", "-")
}

func removeNSServiceAnnotation(client *kubernetes.Clientset, namespace, name string) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": nil,
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = client.CoreV1().Services(namespace).Patch(
		context.TODO(),
		name,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to patch service %s/%s: %w", namespace, name, err)
	}

	return nil
}


func patchNSService(client *kubernetes.Clientset, namespace, name, prefix, username, suffix string) error {
	
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": prefix + username + suffix,
			},
		},
		"spec": map[string]interface{}{
			"type":     string(corev1.ServiceTypeLoadBalancer),
			"selector": nil, // remove any selectors
			"ports": []map[string]interface{}{
				{
					"name":       "ssh-primary",
					"port":       22,
					"targetPort": 22,
					"protocol":   string(corev1.ProtocolTCP),
				},
				{
					"name":       "ssh-secondary",
					"port":       2222,
					"targetPort": 2222,
					"protocol":   string(corev1.ProtocolTCP),
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = client.CoreV1().Services(namespace).Patch(
		context.TODO(),
		name,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to patch service %s/%s: %w", namespace, name, err)
	}

	return nil
}


func createNSService(client *kubernetes.Clientset, prefix, username, suffix string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefix + username,
			Namespace: "default",
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": prefix + username + suffix,
			},
			Labels: map[string]string{
				"cmxsafe.io/managed": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			//Selector: podLabels,
			Type:     corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name:       "ssh-primary",
					Port:       22,
					TargetPort: intstr.FromInt(22),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "ssh-secondary",
					Port:       2222,
					TargetPort: intstr.FromInt(2222),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	_, err := client.CoreV1().Services(namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	return err
}

func createEndpointSlice(client *kubernetes.Clientset, label, username, podIP string) error {
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username + "-slice",
			Namespace: namespace,
			Labels: map[string]string{
				discoveryv1.LabelServiceName: label,
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{podIP},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name:     pointer.String("ssh-primary"),
				Port:     pointer.Int32(22),
				Protocol: (*corev1.Protocol)(pointer.String("TCP")),
			},
			{
				Name:     pointer.String("ssh-secondary"),
				Port:     pointer.Int32(2222),
				Protocol: (*corev1.Protocol)(pointer.String("TCP")),
			},
		},
	}
	_, err := client.DiscoveryV1().EndpointSlices(namespace).Create(context.TODO(), slice, metav1.CreateOptions{})
	return err
}

func createEWService(client *kubernetes.Clientset, namespace, username, podIP string) error {
	ipDash := ipDSH(podIP)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username + "-1883",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.cmxsafe-gw.%s.svc.cluster.local", ipDash, namespace),
		},
	}
	_, err := client.CoreV1().Services(namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	return err
}

func ServiceExists(namespace, name string) bool {
	key := namespace + "/" + "dynamic-service-" + name
	_, exists := serviceCache.Load(key)
	return exists
}

func patchEndpointSlices(client *kubernetes.Clientset, namespace, name, label, newIP string) error {
	sliceClient := client.DiscoveryV1().EndpointSlices(namespace)

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"kubernetes.io/service-name": label,
			},
		},
		"endpoints": []map[string]interface{}{
			{
				"addresses": []string{newIP},
			},
		},
		"ports": []map[string]interface{}{
			{
				"name":     "ssh-primary",
				"port":     22,
				"protocol": "TCP",
			},
			{
				"name":     "ssh-secondary",
				"port":     2222,
				"protocol": "TCP",
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = sliceClient.Patch(context.TODO(), name, types.MergePatchType, patchBytes, metav1.PatchOptions{})

	if err != nil {
		return err
	} else {
		log.Printf("Patched EndpointSlice %s to use IP %s", name, newIP)
	}

	return nil
}

func patchEWService(client *kubernetes.Clientset, namespace, username, podIP string) error {
	ipDash := ipDSH(podIP)
	dns := fmt.Sprintf("%s.cmxsafe-gw.%s.svc.cluster.local", ipDash, namespace)

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"externalName": dns,
		},
	}

	patchBytes, _ := json.Marshal(patch)

	_, err := client.CoreV1().Services(namespace).Patch(
		context.TODO(),
		username,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	return err
}


