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
)

var podIPCache sync.Map
var serviceCache sync.Map

const (
	externalDNSAnnotation = "external-dns.alpha.kubernetes.io/hostname"
	myServiceSuffix       = ".myservices.local"
	serviceNameLabel      = "kubernetes.io/service-name"
	defaultNamespace      = "default"
)


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

func lookupPodIP(namespace, podName string) string {
	key := namespace + "/" + podName
	if val, ok := podIPCache.Load(key); ok {
		return val.(string)
	}
	return "(not cached)"
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

func isManagedLoadBalancer(svc *corev1.Service, annotationKey, suffix string) bool {
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return false
	}
	val, ok := svc.Annotations[annotationKey]
	return ok && strings.HasSuffix(val, suffix)
}

func extractDNSKey(dns, suffix string) string {
	if strings.HasSuffix(dns, suffix) {
		return strings.TrimSuffix(dns, suffix)
	}
	return ""
}

func formatServiceRef(namespace, name string) string {
	return fmt.Sprintf("%s/%s;", namespace, name)
}

func appendToServiceCache(key, value string) {
	existing, ok := serviceCache.Load(key)
	if !ok {
		serviceCache.Store(key, value)
		return
	}
	if !strings.Contains(existing.(string), value) {
		serviceCache.Store(key, existing.(string)+value)
	}
}

func removeFromServiceCache(key, value string) {
	existing, ok := serviceCache.Load(key)
	if !ok {
		return
	}
	updated := strings.ReplaceAll(existing.(string), value, "")
	if strings.TrimSpace(updated) == "" {
		serviceCache.Delete(key)
	} else {
		serviceCache.Store(key, updated)
	}
}


func startServiceInformer(clientset *kubernetes.Clientset, stopCh <-chan struct{}) {
	const annotationKey = "external-dns.alpha.kubernetes.io/hostname"
	const annotationSuffix = ".myservices.local"

	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0)
	informer := factory.Core().V1().Services().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			if !isManagedLoadBalancer(svc, annotationKey, annotationSuffix) {
				return
			}
			key := extractDNSKey(svc.Annotations[annotationKey], annotationSuffix)
			ref := formatServiceRef(svc.Namespace, svc.Name)
			appendToServiceCache(key, ref)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSVC := oldObj.(*corev1.Service)
			newSVC := newObj.(*corev1.Service)

			oldDNS := oldSVC.Annotations[annotationKey]
			newDNS := newSVC.Annotations[annotationKey]
			oldKey := extractDNSKey(oldDNS, annotationSuffix)
			newKey := extractDNSKey(newDNS, annotationSuffix)

			oldRef := formatServiceRef(oldSVC.Namespace, oldSVC.Name)
			newRef := formatServiceRef(newSVC.Namespace, newSVC.Name)

			if oldKey != "" && oldKey != newKey {
				removeFromServiceCache(oldKey, oldRef)
			}
			if newKey != "" {
				appendToServiceCache(newKey, newRef)
			}
		},

		DeleteFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			if !isManagedLoadBalancer(svc, annotationKey, annotationSuffix) {
				return
			}
			key := extractDNSKey(svc.Annotations[annotationKey], annotationSuffix)
			ref := formatServiceRef(svc.Namespace, svc.Name)
			removeFromServiceCache(key, ref)
		},
	})

	factory.Start(stopCh)
	cache.WaitForCacheSync(stopCh, informer.HasSynced)
	log.Println("Service informer started.")
}

func handleEWService(client *kubernetes.Clientset, username, podIP string) error {
	err = patchEWService(clientset, ns, username, podIP)
	if apierrors.IsNotFound(err) {
		err = createEWService(clientset, ns, username, podIP)
	}
	return err
}

func handleEndpoint(client *kubernetes.Clientset, label, username, podIP string) error {
	sliceClient := client.DiscoveryV1().EndpointSlices("")

	labelSelector := fmt.Sprintf("kubernetes.io/service-name=%s", label)
	slices, err := sliceClient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list endpoint slices: %w", err)
	}

	var validSlices []discoveryv1.EndpointSlice

	for _, slice := range slices.Items {
		if containsSSHPort(slice.Ports) {
			validSlices = append(validSlices, slice)
		}
	}

	var selectedSlice *discoveryv1.EndpointSlice
	var conflictsRemoved int

	for _, slice := range validSlices {
		if slice.Namespace == "default" && slice.Name == username+"-slice" {
			selectedSlice = &slice
			continue
		}

		err := removeSliceServiceLabel(client, slice)
		if err != nil {
			log.Printf("Failed to clean up slice %s/%s: %v", slice.Namespace, slice.Name, err)
			continue
		}
		conflictsRemoved++
	}

	if selectedSlice != nil {
		return patchEndpointSlices(client, selectedSlice.Namespace, selectedSlice.Name, label, podIP)
	}

	// No slice was found/retained — create a new one
	return createEndpointSlice(client, label, username, podIP)
}

func containsSSHPort(ports []discoveryv1.EndpointPort) bool {
	for _, port := range ports {
		if port.Port == nil || port.Protocol == nil {
			continue
		}
		switch *port.Port {
		case 22, 2222:
			if *port.Protocol == corev1.ProtocolTCP {
				return true
			}
		}
	}
	return false
}

func removeSliceServiceLabel(client *kubernetes.Clientset, slice discoveryv1.EndpointSlice) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"kubernetes.io/service-name": nil,
			},
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch for %s/%s failed: %w", slice.Namespace, slice.Name, err)
	}

	_, err = client.DiscoveryV1().EndpointSlices(slice.Namespace).Patch(
		context.TODO(),
		slice.Name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("patch failed for slice %s/%s: %w", slice.Namespace, slice.Name, err)
	}
	log.Printf("Removed label from EndpointSlice %s/%s", slice.Namespace, slice.Name)
	return nil
}

func handleNSService(client *kubernetes.Clientset, prefix, username, suffix string) string {
	key := prefix + username
	val, ok := serviceCache.Load(key)
	if !ok {
		val = ""
	}

	serviceList := strings.TrimSuffix(val.(string), ";")
	entries := strings.Split(serviceList, ";")

	// Clean up duplicates
	updatedEntries := []string{}
	for _, entry := range entries {
		if entry == "" || entry == "default/"+prefix+username {
			continue
		}
		parts := strings.SplitN(entry, "/", 2)
		if len(parts) != 2 {
			log.Printf("Invalid service entry: %s", entry)
			continue
		}
		ns, name := parts[0], parts[1]
		err := removeNSServiceAnnotation(client, ns, name)
		if err != nil {
			log.Printf("Failed to remove annotation from service %s/%s: %v", ns, name, err)
			updatedEntries = append(updatedEntries, entry) // keep if we fail to clean up
		}
	}

	// Only one valid entry should remain: default/<prefix><username>
	primary := "default/" + prefix + username
	if len(updatedEntries) == 0 {
		err := createNSService(client, prefix, username, suffix)
		if err != nil {
			log.Printf("Failed to create NS service: %v", err)
			return ""
		}
		return prefix + username
	}

	if len(updatedEntries) == 1 && updatedEntries[0] == primary {
		parts := strings.SplitN(primary, "/", 2)
		err := patchNSService(client, parts[0], parts[1], prefix, username, suffix)
		if err != nil {
			log.Printf("Failed to patch NS service: %v", err)
			return ""
		}
		return parts[1]
	}

	log.Printf("Failed to resolve a single NS service for %s: %v", key, updatedEntries)
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
	hostname := prefix + username + suffix

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"external-dns.alpha.kubernetes.io/hostname": hostname,
			},
		},
		"spec": map[string]interface{}{
			"type":     string(corev1.ServiceTypeLoadBalancer),
			"selector": nil, // ensure no selectors
			"ports": []map[string]interface{}{
				buildPortPatch("ssh-primary", 22),
				buildPortPatch("ssh-secondary", 2222),
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal service patch: %w", err)
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
	name := prefix + username
	hostname := name + suffix
	namespace := "default"

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": hostname,
			},
			Labels: map[string]string{
				"cmxsafe.io/managed": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				buildPort("ssh-primary", 22),
				buildPort("ssh-secondary", 2222),
			},
		},
	}

	_, err := client.CoreV1().Services(namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service %s/%s: %w", namespace, name, err)
	}

	return nil
}

func buildPort(name string, port int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Port:       port,
		TargetPort: intstr.FromInt(int(port)),
		Protocol:   corev1.ProtocolTCP,
	}
}

func buildPortPatch(name string, port int32) map[string]interface{} {
	return map[string]interface{}{
		"name":       name,
		"port":       port,
		"targetPort": port,
		"protocol":   string(corev1.ProtocolTCP),
	}
}

func createEndpointSlice(client *kubernetes.Clientset, label, username, podIP string) error {
	name := username + "-slice"

	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
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
			buildEndpointPort("ssh-primary", 22),
			buildEndpointPort("ssh-secondary", 2222),
		},
	}

	_, err := client.DiscoveryV1().EndpointSlices(namespace).Create(context.TODO(), slice, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create EndpointSlice %s/%s: %w", namespace, name, err)
	}
	log.Printf("Created EndpointSlice %s/%s with IP %s", namespace, name, podIP)
	return nil
}

func buildEndpointPort(name string, port int32) discoveryv1.EndpointPort {
	return discoveryv1.EndpointPort{
		Name:     pointer.String(name),
		Port:     pointer.Int32(port),
		Protocol: (*corev1.Protocol)(pointer.String("TCP")),
	}
}

func createEWService(client *kubernetes.Clientset, namespace, username, podIP string) error {
	ipDash := ipDSH(podIP)
	dns := fmt.Sprintf("%s.cmxsafe-gw.%s.svc.cluster.local", ipDash, namespace)
	name := username + "-1883"

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: dns,
		},
	}

	_, err := client.CoreV1().Services(namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create EW service %s/%s: %w", namespace, name, err)
	}
	log.Printf("Created EW service %s/%s pointing to %s", namespace, name, dns)
	return nil
}

func ServiceExists(namespace, name string) bool {
	key := fmt.Sprintf("%s/dynamic-service-%s", namespace, name)
	_, exists := serviceCache.Load(key)
	return exists
}

func patchEndpointSlices(client *kubernetes.Clientset, namespace, name, label, newIP string) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"kubernetes.io/service-name": label,
			},
		},
		"endpoints": []map[string]interface{}{
			{"addresses": []string{newIP}},
		},
		"ports": []map[string]interface{}{
			buildPortPatch("ssh-primary", 22),
			buildPortPatch("ssh-secondary", 2222),
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal EndpointSlice patch: %w", err)
	}

	_, err = client.DiscoveryV1().EndpointSlices(namespace).Patch(
		context.TODO(), name, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch EndpointSlice %s/%s: %w", namespace, name, err)
	}

	log.Printf("Patched EndpointSlice %s/%s to IP %s", namespace, name, newIP)
	return nil
}

func buildPortPatch(name string, port int32) map[string]interface{} {
	return map[string]interface{}{
		"name":     name,
		"port":     port,
		"protocol": "TCP",
	}
}

func patchEWService(client *kubernetes.Clientset, namespace, username, podIP string) error {
	ipDash := ipDSH(podIP)
	dns := fmt.Sprintf("%s.cmxsafe-gw.%s.svc.cluster.local", ipDash, namespace)

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"externalName": dns,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal EW patch: %w", err)
	}

	name := username + "-1883"
	_, err = client.CoreV1().Services(namespace).Patch(
		context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch EW service %s/%s: %w", namespace, name, err)
	}

	log.Printf("Patched EW service %s/%s to %s", namespace, name, dns)
	return nil
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