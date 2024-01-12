package quota

import (
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/objectcontext"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceObject returns the quota object of a namespace.
func NamespaceObject(ns *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	sns, err := nsutils.SNSFromNamespace(ns)
	if err != nil {
		return nil, err
	}

	return SubnamespaceObject(sns)

}

// RootNSObject returns the quota object of a root namespace.
func RootNSObject(ns *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	quotaObj, err := objectcontext.New(ns.Ctx, ns.Client, client.ObjectKey{Namespace: ns.Name(), Name: ns.Name()}, &corev1.ResourceQuota{})
	if err != nil {
		return quotaObj, err
	}
	return quotaObj, nil
}

// RootNSObjectFromName returns the quota object of a root namespace.
func RootNSObjectFromName(obj *objectcontext.ObjectContext, rootNSName string) (*objectcontext.ObjectContext, error) {
	rootNS, err := objectcontext.New(obj.Ctx, obj.Client, client.ObjectKey{Name: rootNSName}, &corev1.Namespace{})
	if err != nil {
		return nil, err
	}

	rootNSQuotaObj, err := RootNSObject(rootNS)
	if err != nil {
		return nil, err
	}

	return rootNSQuotaObj, nil
}
