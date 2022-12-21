package namespaceDB

//
//import (
//	quotav1 "github.com/openshift/api/quota/v1"
//	corev1 "k8s.io/api/core/v1"
//	"k8s.io/apimachinery/pkg/api/resource"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	v1alpha1 "nodeQuotaSync/api/v1alpha1"
//	"sigs.k8s.io/controller-runtime/pkg/event"
//	"sort"
//	"sync"
//)
//
////this file contains functions and structs that are being used by tests
//
////ResourceDBTemplate is a struct that is used to concat function calls for easier tests
//type ResourceDBTemplate struct {
//	*ResourceDB
//}
//
////ResourceMap a map of resource name and its value (such as {"cpu": "10", "memory": "5Ki"})
//type ResourceMap map[corev1.ResourceName]string
//
////NewResourceDBTemplate initiates a new ResourceDBTemplate to work on
//func NewResourceDBTemplate() ResourceDBTemplate {
//	rdb := ResourceDBTemplate{&ResourceDB{}}
//	rdb.groupResources = make(map[string]NodeResources)
//	rdb.mutex = &sync.RWMutex{}
//	rdb.NodeControllerEvents = make(chan event.GenericEvent)
//	return rdb
//}
//
////GetAllResources returns a nested map contains all groups, the nodes within the group and their resources
//func (rdb ResourceDB) GetAllResources() map[string]NodeResources {
//	return rdb.groupResources
//}
//
////StringSliceSort takes string slice and return the slice sorted
//func StringSliceSort(s []string) []string {
//	sort.Strings(s)
//	return s
//}
//
////AddGroup takes string of a group name and initiates its value's map
//func (rdb ResourceDBTemplate) AddGroup(groupName string) ResourceDBTemplate {
//	rdb.ResourceDB.groupResources[groupName] = make(NodeResources)
//	return rdb
//}
//
////GetResourceList takes resourceMap
////the function returns a resource list contains this values
//func GetResourceList(resources ResourceMap) corev1.ResourceList {
//	rl := map[corev1.ResourceName]resource.Quantity{}
//	for k, v := range resources {
//		rl[k] = resource.MustParse(v)
//	}
//	return rl
//}
//
////AddNode takes group name, node name and resourceMap
////the function adds the node with its resources to the matched group in the db
//func (rdb ResourceDBTemplate) AddNode(groupName string, nodeName string, resources ResourceMap) ResourceDBTemplate {
//	if _, ok := rdb.ResourceDB.groupResources[groupName]; !ok {
//		rdb.ResourceDB.groupResources[groupName] = make(NodeResources)
//	}
//	rdb.ResourceDB.groupResources[groupName][nodeName] = GetResourceList(resources)
//	return rdb
//}
//
////GetNode takes resourceMap, node name and group name
////the function returns v1.Node object contains the given values
//func GetNode(resources ResourceMap, name string, groupName string) *corev1.Node {
//	node := &corev1.Node{
//		TypeMeta: metav1.TypeMeta{
//			Kind:       "Node",
//			APIVersion: "v1",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Name: name,
//		},
//		Spec: corev1.NodeSpec{},
//		Status: corev1.NodeStatus{
//			Allocatable: GetResourceList(resources),
//		},
//	}
//	if groupName != "" {
//		node.ObjectMeta.Labels = make(map[string]string)
//		node.ObjectMeta.Labels[labelSelector] = groupName
//
//	}
//	return node
//}
//
////GetNodeGroup takes slice of resources ({"cpu", "memory"}), nodeGroup name, rootNamespace name, group name, status rootNamespace and label status
////the function returns v1alpha1.NodeGroup object contains the given values
//func GetNodeGroup(resources []corev1.ResourceName, name string, rootNamespaceName string, groupName string, statusRootNamespace string, statusLabel string) *v1alpha1.NodeGroup {
//	nodeGroup := &v1alpha1.NodeGroup{
//		TypeMeta: metav1.TypeMeta{
//			Kind:       "NodeGroup",
//			APIVersion: "v1alpha1",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Name: name,
//		},
//		Spec: v1alpha1.NodeGroupSpec{
//			LabelSelector: map[string]string{labelSelector: groupName},
//			RootNamespace: rootNamespaceName,
//			Resources:     resources,
//		},
//		Status: v1alpha1.NodeGroupStatus{
//			GroupResources: initiateEmptyRLStatus(resources),
//			LabelSelector:  map[string]string{labelSelector: statusLabel},
//			RootNamespace:  statusRootNamespace,
//			Nodes:          nil,
//		},
//	}
//	return nodeGroup
//}
//
////initiateEmptyRLStatus takes slice of resource names
////the function returns a resource list contains the resources and their zero values
//func initiateEmptyRLStatus(resources []corev1.ResourceName) corev1.ResourceList {
//	rl := corev1.ResourceList{}
//	for _, resourceName := range resources {
//		rl[resourceName] = resource.MustParse("0")
//	}
//	return rl
//}
//
////GetClusterResourceQuota takes cluster resource quota name, cpu count and memory count
////the function returns a quotav1.ClusterResourceQuota with the given values
//func GetClusterResourceQuota(name string, cpu string, memory string) *quotav1.ClusterResourceQuota {
//	return &quotav1.ClusterResourceQuota{
//		TypeMeta: metav1.TypeMeta{
//			Kind:       "ClusterResourceQuota",
//			APIVersion: "quota.openshift.io/v1",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Name:   name,
//			Labels: map[string]string{"crq.subnamespace": name},
//		},
//		Spec: quotav1.ClusterResourceQuotaSpec{
//			Selector: quotav1.ClusterResourceQuotaSelector{
//				AnnotationSelector: map[string]string{"dana.hns.io/crq-selector-0": name},
//			},
//			Quota: corev1.ResourceQuotaSpec{
//				Hard: getCRQResourcelist(cpu, memory),
//			},
//		},
//		Status: quotav1.ClusterResourceQuotaStatus{},
//	}
//}
//
////getCRQResourcelist takes cpu count and memory count
////the function return a resource list contains kubernetes objects default limitations and cpu, memory limitations as given
//func getCRQResourcelist(cpu string, memory string) corev1.ResourceList {
//	rl := corev1.ResourceList{"basic.storageclass.storage.k8s.io/requests.storage": resource.MustParse("70Ti"),
//		"configmaps":                            resource.MustParse("10000"),
//		"count/buildconfigs.build.openshift.io": resource.MustParse("10000"),
//		"count/builds.build.openshift.io":       resource.MustParse("10000"),
//		"count/cronjobs.batch":                  resource.MustParse("10000"),
//		"count/daemonsets.apps":                 resource.MustParse("10000"),
//		"count/persistentvolumeclaims":          resource.MustParse("10000"),
//		"count/replicasets.apps":                resource.MustParse("10000"),
//		"count/replicationcontrollers":          resource.MustParse("10000"),
//		"count/routes.route.openshift.io":       resource.MustParse("10000"),
//		"count/secrets":                         resource.MustParse("10000"),
//		"count/serviceaccounts":                 resource.MustParse("10000"),
//		"count/services":                        resource.MustParse("10000"),
//		"count/statefulsets.apps":               resource.MustParse("10000"),
//		"pods":                                  resource.MustParse("28000"),
//		"services.nodeports":                    resource.MustParse("10000"),
//		"cpu":                                   resource.MustParse(cpu),
//		"memory":                                resource.MustParse(memory),
//		"requests.nvidia.com/gpu":               resource.MustParse("0"),
//	}
//	return rl
//}
