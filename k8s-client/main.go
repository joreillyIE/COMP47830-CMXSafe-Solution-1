package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tetragon "github.com/cilium/tetragon/api/v1/tetragon"
	"k8s.io/utils/pointer"
)

var (
	podIPCache     sync.Map
	serviceCache   sync.Map
	defaultNS      = "default"
	externalDNSAnn = "external-dns.alpha.kubernetes.io/hostname"
	serviceSuffix  = ".myservices.local"
	servicePrefix = "dynamic-service-"
)

func connectTetragonGRPC() *grpc.ClientConn {
	socket := "/var/run/tetragon/tetragon.sock"
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return net.Dial("unix", socket)
	}
	conn, err := grpc.DialContext(context.Background(), "unix://"+socket,
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("tetragon dial error: %v", err)
	}
	return conn
}

func setupKubeClient() *kubernetes.Clientset {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		kc := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		cfg, err = clientcmd.BuildConfigFromFlags("", kc)
		if err != nil {
			log.Fatalf("build config: %v", err)
		}
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("new client: %v", err)
	}
	return cs
}

/* ---------- Informers (Pods + LB Services) ---------- */

func startPodInformer(cs *kubernetes.Clientset, stop <-chan struct{}) {
	f := informers.NewSharedInformerFactoryWithOptions(cs, 0,
		informers.WithNamespace(defaultNS),
		informers.WithTweakListOptions(func(o *metav1.ListOptions) {
			o.LabelSelector = "app=cmxsafe-gw"
		}))
	inf := f.Core().V1().Pods().Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			p := o.(*corev1.Pod)
			podIPCache.Store(p.Namespace+"/"+p.Name, p.Status.PodIP)
		},
		UpdateFunc: func(_, n interface{}) {
			p := n.(*corev1.Pod)
			podIPCache.Store(p.Namespace+"/"+p.Name, p.Status.PodIP)
		},
		DeleteFunc: func(o interface{}) {
			p := o.(*corev1.Pod)
			podIPCache.Delete(p.Namespace + "/" + p.Name)
		},
	})
	f.Start(stop)
	cache.WaitForCacheSync(stop, inf.HasSynced)
}

func startServiceInformer(cs *kubernetes.Clientset, stop <-chan struct{}) {
	f := informers.NewSharedInformerFactory(cs, 0)
	inf := f.Core().V1().Services().Informer()

	isManaged := func(s *corev1.Service) bool {
		if s.Spec.Type != corev1.ServiceTypeLoadBalancer {
			return false
		}
		if v := s.Annotations[externalDNSAnn]; v != "" && strings.HasSuffix(v, serviceSuffix) {
			return true
		}
		return false
	}

	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			s := o.(*corev1.Service)
			if !isManaged(s) {
				return
			}
			key := strings.TrimSuffix(s.Annotations[externalDNSAnn], serviceSuffix)
			ref := fmt.Sprintf("%s/%s;", s.Namespace, s.Name)
			appendToServiceCache(key, ref)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldS := oldObj.(*corev1.Service)
			newS := newObj.(*corev1.Service)
			oldKey := strings.TrimSuffix(oldS.Annotations[externalDNSAnn], serviceSuffix)
			newKey := strings.TrimSuffix(newS.Annotations[externalDNSAnn], serviceSuffix)
			oldRef := fmt.Sprintf("%s/%s;", oldS.Namespace, oldS.Name)
			newRef := fmt.Sprintf("%s/%s;", newS.Namespace, newS.Name)
			if oldKey != "" && oldKey != newKey {
				removeFromServiceCache(oldKey, oldRef)
			}
			if newKey != "" {
				appendToServiceCache(newKey, newRef)
			}
		},
		DeleteFunc: func(o interface{}) {
			s := o.(*corev1.Service)
			if !isManaged(s) {
				return
			}
			key := strings.TrimSuffix(s.Annotations[externalDNSAnn], serviceSuffix)
			ref := fmt.Sprintf("%s/%s;", s.Namespace, s.Name)
			removeFromServiceCache(key, ref)
		},
	})
	f.Start(stop)
	cache.WaitForCacheSync(stop, inf.HasSynced)
}

/* ---------- North‑South & East‑West helpers ---------- */

func buildPort(name string, p int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name: name, Port: p,
		TargetPort: intstr.FromInt(int(p)),
		Protocol:   corev1.ProtocolTCP,
	}
}
func buildPortPatch(name string, p int32) map[string]interface{} {
	return map[string]interface{}{
		"name": name, "port": p, "targetPort": p,
		"protocol": string(corev1.ProtocolTCP),
	}
}
func buildEndpointPort(name string, p int32) discoveryv1.EndpointPort {
	return discoveryv1.EndpointPort{
		Name:     pointer.String(name),
		Port:     pointer.Int32(p),
		Protocol: (*corev1.Protocol)(pointer.String("TCP")),
	}
}
func buildEndpointPortPatch(name string, p int32) map[string]interface{} {
	return map[string]interface{}{
		"name": name, "port": p,
		"protocol": string(corev1.ProtocolTCP),
	}
}

/* ---------- East‑West service ---------- */
func patchEWService(cs *kubernetes.Clientset, ns, user, ip string) error {
	host := fmt.Sprintf("%s.cmxsafe-gw.%s.svc.cluster.local", strings.ReplaceAll(ip, ".", "-"), ns)
	patch := map[string]interface{}{"spec": map[string]interface{}{"externalName": host}}
	b, _ := json.Marshal(patch)
	_, err := cs.CoreV1().Services(ns).Patch(context.TODO(),
		user, types.StrategicMergePatchType, b, metav1.PatchOptions{})
	return err
}
func createEWService(cs *kubernetes.Clientset, ns, user, ip string) error {
	host := fmt.Sprintf("%s.cmxsafe-gw.%s.svc.cluster.local", strings.ReplaceAll(ip, ".", "-"), ns)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: user, Namespace: ns},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: host},
	}
	_, err := cs.CoreV1().Services(ns).Create(context.TODO(), svc, metav1.CreateOptions{})
	return err
}
func handleEWService(cs *kubernetes.Clientset, ns, user, ip string) error {
	if e := patchEWService(cs, ns, user, ip); apierrors.IsNotFound(e) {
		return createEWService(cs, ns, user, ip)
	} else {
		return e
	}
}

/* ---------- EndpointSlice helpers ---------- */

func containsSSH(ports []discoveryv1.EndpointPort) bool {
	for _, p := range ports {
		if p.Port == nil || p.Protocol == nil {
			continue
		}
		if *p.Protocol == corev1.ProtocolTCP && (*p.Port == 22 || *p.Port == 2222) {
			return true
		}
	}
	return false
}
func removeSliceLabel(cs *kubernetes.Clientset, s discoveryv1.EndpointSlice) error {
	patch := map[string]interface{}{"metadata": map[string]interface{}{
		"labels": map[string]interface{}{"kubernetes.io/service-name": nil}}}
	b, _ := json.Marshal(patch)
	_, err := cs.DiscoveryV1().EndpointSlices(s.Namespace).
		Patch(context.TODO(), s.Name, types.MergePatchType, b, metav1.PatchOptions{})
	return err
}
func createEndpointSlice(cs *kubernetes.Clientset, label, user, ip string) error {
	name := user + "-slice"
	s := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name, Namespace: defaultNS,
			Labels:    map[string]string{discoveryv1.LabelServiceName: label},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{ip}}},
		Ports: []discoveryv1.EndpointPort{
			buildEndpointPort("ssh-primary", 22),
			buildEndpointPort("ssh-secondary", 2222),
		},
	}
	_, err := cs.DiscoveryV1().EndpointSlices(defaultNS).
		Create(context.TODO(), s, metav1.CreateOptions{})
	return err
}
func patchEndpointSlice(cs *kubernetes.Clientset, ns, name, label, ip string) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{"kubernetes.io/service-name": label},
		},
		"endpoints": []map[string]interface{}{{"addresses": []string{ip}}},
		"ports": []map[string]interface{}{
			buildEndpointPortPatch("ssh-primary", 22),
			buildEndpointPortPatch("ssh-secondary", 2222)},
	}
	b, _ := json.Marshal(patch)
	_, err := cs.DiscoveryV1().EndpointSlices(ns).
		Patch(context.TODO(), name, types.MergePatchType, b, metav1.PatchOptions{})
	return err
}

/* ---------- helper functions ---------- */

// cache helpers
func appendToServiceCache(key, val string) {
	if cur, ok := serviceCache.Load(key); !ok {
		serviceCache.Store(key, val)
	} else if !strings.Contains(cur.(string), val) {
		serviceCache.Store(key, cur.(string)+val)
	}
}
func removeFromServiceCache(key, val string) {
	if cur, ok := serviceCache.Load(key); ok {
		newVal := strings.ReplaceAll(cur.(string), val, "")
		if strings.TrimSpace(newVal) == "" {
			serviceCache.Delete(key)
		} else {
			serviceCache.Store(key, newVal)
		}
	}
}

// simple look‑ups / parsing
func lookupPodIP(ns, pod string) string {
	if v, ok := podIPCache.Load(ns + "/" + pod); ok {
		return v.(string)
	}
	return ""
}
func extractUsername(cwd string) string {
	parts := strings.Split(cwd, "/")
	if len(parts) >= 3 && parts[1] == "home" {
		return parts[2]
	}
	return ""
}

/* ---------- handle NS Services ---------- */
func handleNSService(
	cs *kubernetes.Clientset,
	prefix, user, suffix string) string {

	key := prefix + user
	raw, _ := serviceCache.LoadOrStore(key, "")
	list := strings.TrimSuffix(raw.(string), ";")
	entries := strings.Split(list, ";")

	primary := defaultNS + "/" + prefix + user

	// remove duplicates
	tmp := entries[:0]
	for _, e := range entries {
		if e == "" || e == primary {
			continue
		}
		nsName := strings.SplitN(e, "/", 2)
		if len(nsName) != 2 {
			continue
		}
		if err := removeNSServiceAnnotation(cs, nsName[0], nsName[1]); err != nil {
			tmp = append(tmp, e) // keep if patch failed
		}
	}
	entries = tmp
	if len(entries) == 0 {
		_ = createNSService(cs, prefix, user, suffix)
		appendToServiceCache(key, primary+";")
		return prefix + user
	}
	if len(entries) == 1 && entries[0] == primary {
		nsName := strings.SplitN(primary, "/", 2)
		_ = patchNSService(cs, nsName[0], nsName[1], prefix, user, suffix)
		return nsName[1]
	}
	return ""
}

// --- helpers ---

const externalDNSAnnotation = "external-dns.alpha.kubernetes.io/hostname"

func removeNSServiceAnnotation(
	cs *kubernetes.Clientset, ns, name string) error {

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				externalDNSAnnotation: nil,
			},
		},
	}
	b, _ := json.Marshal(patch)
	_, err := cs.CoreV1().Services(ns).Patch(
		context.TODO(), name,
		types.StrategicMergePatchType, b, metav1.PatchOptions{})
	return err
}

func createNSService(
	cs *kubernetes.Clientset, prefix, user, suffix string) error {

	name := prefix + user
	host := name + suffix
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNS,
			Annotations: map[string]string{
				externalDNSAnnotation: host,
			},
			Labels: map[string]string{"cmxsafe.io/managed": "true"},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				buildPort("ssh-primary", 22),
				buildPort("ssh-secondary", 2222),
			},
		},
	}
	_, err := cs.CoreV1().Services(defaultNS).
		Create(context.TODO(), svc, metav1.CreateOptions{})
	return err
}

func patchNSService(
	cs *kubernetes.Clientset, ns, name, prefix, user, suffix string) error {

	host := prefix + user + suffix
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				externalDNSAnnotation: host,
			},
		},
		"spec": map[string]interface{}{
			"type":     string(corev1.ServiceTypeLoadBalancer),
			"selector": nil,
			"ports": []map[string]interface{}{
				buildPortPatch("ssh-primary", 22),
				buildPortPatch("ssh-secondary", 2222),
			},
		},
	}
	b, _ := json.Marshal(patch)
	_, err := cs.CoreV1().Services(ns).Patch(
		context.TODO(), name,
		types.StrategicMergePatchType, b, metav1.PatchOptions{})
	return err
}

/* ---------- MAIN ---------- */

func main() {
	stop := make(chan struct{})
	cs := setupKubeClient()
	go startPodInformer(cs, stop)
	go startServiceInformer(cs, stop)

	conn := connectTetragonGRPC()
	defer conn.Close()

	cli := tetragon.NewFineGuidanceSensorsClient(conn)
	stream, err := cli.GetEvents(context.Background(), &tetragon.GetEventsRequest{
		AllowList: []*tetragon.Filter{{
			EventSet:       []tetragon.EventType{tetragon.EventType_PROCESS_EXEC},
			ArgumentsRegex: []string{".*createservice\\.sh.*"},
			PodRegex:       []string{`^cmxsafe-gw-.*`},
			BinaryRegex:    []string{`^/usr/bin/logger$`},
		}},
	})
	if err != nil {
		log.Fatalf("GetEvents: %v", err)
	}

	for {
		r, e := stream.Recv()
		if e != nil {
			log.Fatalf("stream closed: %v", e)
		}
		ex := r.GetProcessExec()
		if ex == nil {
			continue
		}
		p := ex.Process
		podName, ns := p.Pod.Name, p.Pod.Namespace
		ip := lookupPodIP(ns, podName)
		user := extractUsername(p.Cwd)
		if ip == "" || user == "" {
			continue
		}

		// North‑South LB service label
		label := handleNSService(cs, servicePrefix, user, serviceSuffix)
		if label == "" {
			continue
		}
		if err := patchEndpointSlice(cs, defaultNS, user+"-slice", label, ip); err != nil {
			if apierrors.IsNotFound(err) {
				_ = createEndpointSlice(cs, label, user, ip)
			} else { log.Print(err) }
		}
		if err := handleEWService(cs, ns, user, ip); err != nil {
			log.Print(err)
		}
	}
}
