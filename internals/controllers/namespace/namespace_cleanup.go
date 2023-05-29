package controllers

import (
	"context"
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// cleanUp takes care of clean-up related operations that need to be done when
// a namespace created by the HNS is deleted
func (r *NamespaceReconciler) cleanUp(ctx context.Context, nsObject *utils.ObjectContext) error {
	logger := log.FromContext(ctx)
	logger.Info("cleaning up namespace")
	nsName := nsObject.Object.GetName()

	if err := r.deleteNamespaceFromNamespaceDB(nsName); err != nil {
		return fmt.Errorf("failed to delete namespace '%s' from namespaceDB: "+err.Error(), nsName)
	}
	logger.Info("successfully deleted namespace from namespaceDB", "namespace", nsName)

	if err := deleteNamespaceQuotaObject(nsObject); err != nil {
		return fmt.Errorf("failed to delete quota object of namespace '%s': "+err.Error(), nsName)
	}
	logger.Info("successfully deleted quota object of namespace", "namespace", nsName)

	if err := deleteSNSFromParentNS(nsName, nsObject); err != nil {
		return fmt.Errorf("failed to delete subnamespace object '%s' from parent namespace: "+err.Error(), nsName)
	}
	logger.Info("successfully deleted subnamespace object from parent namespace", "namespace", nsName)

	if err := deleteNSHnsView(nsObject); err != nil {
		return fmt.Errorf("failed to delete role and rolebinding objects associated with namespace '%s': "+err.Error(), nsName)
	}
	logger.Info("successfully deleted role and rolebinding objects associated with namespace", "namespace", nsName)

	// trigger reconciliation for parent subnamespace so that it can be aware of
	// potential changes in one of its children
	if err := r.enqueueParentSNSEvent(nsObject); err != nil {
		return fmt.Errorf("failed to trigger sns event for parent of namespace '%s': "+err.Error(), nsName)
	}
	logger.Info("successfully triggered sns event for parent of namespace", "namespace", nsName)

	if err := deleteNSFinalizer(nsObject); err != nil {
		return fmt.Errorf("failed to delete finalizer of namespace '%s': "+err.Error(), nsName)
	}
	logger.Info("successfully deleted finalizer of namespace", "namespace", nsName)

	return nil
}

// deleteNSHnsView deletes the cluster role and cluster role binding HNS objects
// associated with the namespace
func deleteNSHnsView(nsObject *utils.ObjectContext) error {
	nsHnsViewClusterRoleObj, nsHnsViewClusterRoleBindingObj, err := getNamespaceHNSView(nsObject)
	if err != nil {
		return err
	}

	if err := nsHnsViewClusterRoleObj.EnsureDeleteObject(); err != nil {
		return err
	}

	return nsHnsViewClusterRoleBindingObj.EnsureDeleteObject()
}

// deleteNamespaceQuotaObject deletes the quota object corresponding to the subnamespace which
// exists for a given namespace
func deleteNamespaceQuotaObject(ns *utils.ObjectContext) error {
	sns, err := utils.NewObjectContext(ns.Ctx, ns.Client, types.NamespacedName{Name: utils.GetNamespaceSNSPointerAnnotation(ns.Object),
		Namespace: utils.GetNamespaceParent(ns.Object)}, &danav1.Subnamespace{})
	if err != nil {
		return err
	}

	quotaObject, err := utils.GetNSQuotaObject(sns)
	if err != nil {
		return err
	}

	if err := quotaObject.EnsureDeleteObject(); err != nil {
		return err
	}
	return nil
}

// deleteNamespaceFromNamespaceDB deletes the given namespace from the namespaceDB
// if the namespace is a key, then remove the key from the DB; otherwise remove
// only the namespace from the list of namespaces under a particular key
func (r *NamespaceReconciler) deleteNamespaceFromNamespaceDB(nsName string) error {
	keyNS := r.NamespaceDB.GetKey(nsName)

	if keyNS != "" {
		if keyNS == nsName {
			r.NamespaceDB.DeleteKey(keyNS)
		} else {
			if err := r.NamespaceDB.RemoveNS(nsName, keyNS); err != nil {
				return fmt.Errorf("failed to remove namespace '%s' from key in DB: "+err.Error(), nsName)
			}
		}
	}

	return nil
}

// enqueueParentSNSEvent enqueues subanmespace event for the parent of the namespace
func (r *NamespaceReconciler) enqueueParentSNSEvent(nsObject *utils.ObjectContext) error {
	nsParentName := utils.GetNamespaceParent(nsObject.Object)

	nsParentNSObj, err := utils.NewObjectContext(nsObject.Ctx, r.Client, types.NamespacedName{Name: nsParentName}, &corev1.Namespace{})
	if err != nil {
		return err
	}

	nsGrandparentName := utils.GetNamespaceParent(nsParentNSObj.Object)

	r.SNSEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsParentName,
			Namespace: nsGrandparentName,
		},
	}}

	return nil
}

// deleteSNSFromParentNS deletes the subnamespace object from the parent namespace of the SNS
func deleteSNSFromParentNS(nsName string, nsObject *utils.ObjectContext) error {
	nsParentName := utils.GetNamespaceParent(nsObject.Object)

	sns, err := utils.NewObjectContext(nsObject.Ctx, nsObject.Client, types.NamespacedName{Name: nsName, Namespace: nsParentName}, &danav1.Subnamespace{})
	if err != nil {
		return err
	}
	return sns.EnsureDeleteObject()
}

// deleteNSFinalizer removes the HNS finalizer from the namespace
func deleteNSFinalizer(nsObject *utils.ObjectContext) error {
	return nsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("removed namespace finalizer", danav1.NsFinalizer)
		controllerutil.RemoveFinalizer(object, danav1.NsFinalizer)
		return object, log
	})
}
