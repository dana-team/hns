package utils

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsChildlessNamespace(namespace *ObjectContext) bool {
	if !isNamespace(namespace.Object) {
		return false
	}
	snsList, err := NewObjectContextList(namespace.Ctx, namespace.Log, namespace.Client, &danav1.SubnamespaceList{}, client.InNamespace(namespace.Object.GetName()))
	if err != nil {
		return false
	}
	if len(snsList.Objects.(*danav1.SubnamespaceList).Items) == 0 {
		return true
	}
	return false
}

// IsRq returns true if the depth of the subnamespace is less or equal
// the pre-set rqDepth AND if the subnamespace is not a ResourcePool
func IsRq(sns *ObjectContext, offset int) (bool, error) {
	rootRQDepth, err := GetRqDepthFromSNS(sns)
	if err != nil {
		return false, err
	}
	nsDepth, err := GetSnsDepth(sns)
	if err != nil {
		return false, err
	}

	rootRQDepthInt, _ := strconv.Atoi(rootRQDepth)
	nsDepthInt, _ := strconv.Atoi(nsDepth)

	depthFlag := (nsDepthInt + offset) <= rootRQDepthInt
	if offset == danav1.ParentOffset {
		return depthFlag, nil
	}

	resourcePoolFlag := GetSnsResourcePooled(sns.Object) == "false"
	return depthFlag && resourcePoolFlag, nil
}

func GetSnsDepth(sns *ObjectContext) (string, error) {
	ownerNamespace, err := NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: GetSnsOwner(sns.Object)}, &corev1.Namespace{})
	if err != nil {
		return "", err
	}

	if !ownerNamespace.IsPresent() {
		err := errors.New("owner namespace missing")
		sns.Log.Error(err, "subspace owner namespace not found")
		return "", err

	}
	depth, _ := ownerNamespace.Object.GetAnnotations()[danav1.Depth]
	depthInt, _ := strconv.Atoi(depth)
	depthInt = depthInt + 1
	return strconv.Itoa(depthInt), nil
}

func GetRqDepthFromSNS(sns *ObjectContext) (string, error) {
	rootns := corev1.Namespace{}
	parentns := corev1.Namespace{}
	if err := sns.Client.Get(sns.Ctx, types.NamespacedName{Name: sns.Object.GetNamespace()}, &parentns); err != nil {
		return "", err
	}
	if err := sns.Client.Get(sns.Ctx, types.NamespacedName{Name: parentns.Annotations[danav1.RootCrqSelector]}, &rootns); err != nil {
		return "", err
	}
	return rootns.Annotations[danav1.RqDepth], nil
}

func GetRqDepthFromNS(ns *ObjectContext) (string, error) {
	rootns := corev1.Namespace{}
	if err := ns.Client.Get(ns.Ctx, types.NamespacedName{Name: ns.Object.GetAnnotations()[danav1.RootCrqSelector]}, &rootns); err != nil {
		return "", err
	}
	return rootns.Annotations[danav1.RqDepth], nil
}

func IsServiceAccount(roleBinding client.Object) bool {
	if !IsValidRoleBinding(roleBinding) {
		return false
	}
	rbKind := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Kind
	if rbKind != "ServiceAccount" {
		return false
	}
	return true
}

func DeletionTimeStampExists(object client.Object) bool {
	return !object.GetDeletionTimestamp().IsZero()
}

//func ComposeUpdateQuota(name string, operand danav1.Operand, parentns string, childns string, quota corev1.ResourceList) *danav1.UpdateQuota {
//	return &danav1.UpdateQuota{
//		ObjectMeta: metav1.ObjectMeta{
//			Name: name,
//		},
//		Spec: danav1.UpdateQuotaSpec{
//			SourceNamespace:   parentns,
//			DestNamespace:     childns,
//			Operand:           operand,
//			ResourceQuotaSpec: corev1.ResourceQuotaSpec{Hard: quota}},
//	}
//}

func UsernameToFilter(username string) bool {
	invalidNames := [...]string{"system:serviceaccount:default-rolebindings-controller", "system:serviceaccount:sns-system:default"}
	for _, name := range invalidNames {
		if strings.Contains(username, name) {
			return true
		}
	}
	return false
}

func GetClusterName(ctx context.Context, logger logr.Logger, routeClient client.Client) (string, error) {
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	})
	consoleRoute, err := NewObjectContext(ctx, logger, routeClient, types.NamespacedName{Name: "console", Namespace: "openshift-console"}, route)
	if err != nil {
		return "", err
	}
	clusterHost, found, err := unstructured.NestedString(consoleRoute.Object.(*unstructured.Unstructured).Object, "spec", "host")
	if !found || err != nil {
		return clusterHost, err
	}
	clusterName := strings.Split(clusterHost, ".")[2]
	return clusterName, nil
}

func UpdateAllNsChildsOfNs(nsparent *ObjectContext) error {
	subspaceChilds, err := NewObjectContextList(nsparent.Ctx, nsparent.Log, nsparent.Client, &danav1.SubnamespaceList{}, client.InNamespace(nsparent.Object.GetName()))
	if err != nil {
		return err
	}
	for _, sns := range subspaceChilds.Objects.(*danav1.SubnamespaceList).Items {
		ns, _ := NewObjectContext(nsparent.Ctx, nsparent.Log, nsparent.Client, types.NamespacedName{Name: sns.GetName()}, &corev1.Namespace{})
		err := ns.UpdateNsByparent(nsparent, ns)
		if err != nil {
			return err
		}
		return UpdateAllNsChildsOfNs(ns)
	}
	return nil
}

func IndexOf(element string, arr []string) (int, error) {
	for key, value := range arr {
		if element == value {
			return key, nil
		}
	}
	return -1, fmt.Errorf("dont find root ns")
}
