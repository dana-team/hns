/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	quotav1 "github.com/openshift/api/quota/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"time"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/namespaceDB"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// MigrationHierarchyReconciler reconciles a MigrationHierarchy object
type MigrationHierarchyReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	NamespaceDB *namespaceDB.NamespaceDB
	SnsEvents   chan event.GenericEvent
}

// +kubebuilder:rbac:groups=dana.hns.io,resources=migrationhierarchies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dana.hns.io,resources=migrationhierarchies/status,verbs=get;update;patch

func (r *MigrationHierarchyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&danav1.MigrationHierarchy{}).
		Complete(r)
}

func (r *MigrationHierarchyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("controllers").WithName("MigrationHierarchy").WithValues("mh", req.NamespacedName)
	logger.Info("starting to reconcile")

	mhObject, err := utils.NewObjectContext(ctx, r.Client, client.ObjectKey{Name: req.NamespacedName.Name}, &danav1.MigrationHierarchy{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get object '%s': "+err.Error(), mhObject.Object.GetName())
	}

	if !mhObject.IsPresent() {
		logger.Info("resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	phase := mhObject.Object.(*danav1.MigrationHierarchy).Status.Phase
	if utils.ShouldReconcile(phase) {
		return ctrl.Result{}, r.reconcile(mhObject)
	} else {
		logger.Info("no need to reconcile, object phase is: ", "phase", phase)
	}

	return ctrl.Result{}, nil
}

func (r *MigrationHierarchyReconciler) reconcile(mhObject *utils.ObjectContext) error {
	ctx := mhObject.Ctx
	logger := log.FromContext(ctx)

	currentNamespace := mhObject.Object.(*danav1.MigrationHierarchy).Spec.CurrentNamespace
	toNamespace := mhObject.Object.(*danav1.MigrationHierarchy).Spec.ToNamespace

	ns, err := utils.NewObjectContext(ctx, r.Client, client.ObjectKey{Namespace: "", Name: currentNamespace}, &corev1.Namespace{})
	if err != nil {
		return fmt.Errorf("failed getting namespace object '%s': "+err.Error(), currentNamespace)
	}

	sourceSNSParentName := utils.GetNamespaceParent(ns.Object)
	oldSNS, err := utils.NewObjectContext(ctx, r.Client, client.ObjectKey{Name: currentNamespace, Namespace: sourceSNSParentName}, &danav1.Subnamespace{})
	if err != nil {
		return fmt.Errorf("failed getting subnamespace object '%s': "+err.Error(), sourceSNSParentName)
	}

	newSNS, err := r.createNewSNS(oldSNS, mhObject, currentNamespace, toNamespace)
	if err != nil {
		return fmt.Errorf("failed creating subnamespace '%s' under namespace '%s': "+err.Error(), currentNamespace, toNamespace)
	}
	logger.Info("successfully created new subnamespace under new parent", "subnamespace", currentNamespace, "new parent", toNamespace)

	if err := r.deleteOldSNS(oldSNS, newSNS, mhObject); err != nil {
		return fmt.Errorf("failed deleting old subnamespace '%s' :"+err.Error(), oldSNS.GetName())
	}
	logger.Info("successfully deleted old subnamespace from old parent", "subnamesapce", currentNamespace, "old parent", sourceSNSParentName)

	toNS, er := utils.NewObjectContext(ctx, r.Client, client.ObjectKey{Namespace: "", Name: toNamespace}, &corev1.Namespace{})
	if er != nil {
		err := r.updateMHErrorStatus(mhObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), toNamespace)
	}

	if err := r.updateRelatedObjects(mhObject, toNS, ns); err != nil {
		return fmt.Errorf("failed to update related objects: " + err.Error())
	}
	logger.Info("successfully updated related objects of subnamespace", "subnamespace", currentNamespace)

	// after the migration is completed, we need to update the db to account for the new parent
	// MigrateNsHierarchy updates the namespace and its children hierarchy to be under the new parent in the DB
	if err := namespaceDB.MigrateNSHierarchy(ctx, r.NamespaceDB, r.Client, ns.GetName(), toNS.GetName()); err != nil {
		err := r.updateMHErrorStatus(mhObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed migrating subnamespace '%s' in namespaceDB: "+err.Error(), ns.GetName())
	}
	logger.Info("successfully migrated subnamespace in namespaceDB", "subnamespace", currentNamespace)

	// enqueue for reconciliation the original parent of the subnamespace that should be migrated in order for
	// the old parent's status to show the now-changed list of child subnamespaces
	if err := r.enqueueOriginalParent(ctx, sourceSNSParentName); err != nil {
		err := r.updateMHErrorStatus(mhObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed to enqueue '%s': "+err.Error(), sourceSNSParentName)
	}
	logger.Info("successfully enqueued original parent namespace", "oldParent", sourceSNSParentName)

	// enqueue for reconciliation the descendants of the subnamespace so that their labels and annotations
	// are updated properly
	r.enqueueSNSDescendants(newSNS)
	logger.Info("successfully enqueued descendants of subnamespace", "subnamespace", currentNamespace)

	if err := r.updateMHSuccessStatus(mhObject); err != nil {
		err := r.updateMHErrorStatus(mhObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
	}
	logger.Info("successfully updated status of MigrationHierarchy object")

	return nil
}

// updateRelatedObjects handles the update of objects related to the
// migrated subnamesapce such as its namespace and its children
func (r *MigrationHierarchyReconciler) updateRelatedObjects(mhObject, toNS, ns *utils.ObjectContext) error {
	ctx := mhObject.Ctx

	if er := r.UpdateNSBasedOnParent(ctx, toNS, ns); er != nil {
		err := r.updateMHErrorStatus(mhObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed updating the labels and annotations of namespace '%s' according to its parent '%s': "+err.Error(), ns.GetName(), toNS.GetName())
	}

	if er := r.UpdateAllNSChildrenOfNs(ctx, ns); er != nil {
		err := r.updateMHErrorStatus(mhObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed updating labels and annotations of child namespaces of sunamespace '%s': "+err.Error(), ns.GetName())
	}

	if er := r.updateRole(toNS, danav1.NoRole); er != nil {
		err := r.updateMHErrorStatus(mhObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed updating role of subnamespace '%s': "+err.Error(), toNS.GetName())
	}

	return nil
}

// createNewSNS handles the creation of the migrated subnamespace under a new parent
func (r *MigrationHierarchyReconciler) createNewSNS(sns, mhObject *utils.ObjectContext, currentNamespace, toNamespace string) (*utils.ObjectContext, error) {
	newSNS, er := r.createSNS(sns, currentNamespace, toNamespace)
	if er != nil {
		err := r.updateMHErrorStatus(mhObject, er)
		if err != nil {
			return nil, fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return nil, er
	}

	return newSNS, nil
}

// deleteOldSNS handles the deleting the migrated subnamespace
func (r *MigrationHierarchyReconciler) deleteOldSNS(oldSNS, newSNS, mhObject *utils.ObjectContext) error {
	if er := r.deleteSNS(oldSNS); er != nil {
		err := r.updateMHErrorStatus(mhObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}

		// if deleting the old subnamespace fails, delete the newly created subnamespace
		if er := r.deleteSNS(newSNS); er != nil {
			err := r.updateMHErrorStatus(mhObject, er)
			if err != nil {
				return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
			}
			return fmt.Errorf("failed deleting subnamespace '%s' :"+err.Error(), newSNS.GetName())
		}
	}

	return nil
}

// UpdateAllNSChildrenOfNs updates all the children namespaces of a parent namespace recursively
func (r *MigrationHierarchyReconciler) UpdateAllNSChildrenOfNs(cxt context.Context, parentNS *utils.ObjectContext) error {
	snsChildren, err := utils.NewObjectContextList(cxt, parentNS.Client, &danav1.SubnamespaceList{}, client.InNamespace(parentNS.Object.GetName()))
	if err != nil {
		return err
	}

	for _, sns := range snsChildren.Objects.(*danav1.SubnamespaceList).Items {
		ns, _ := utils.NewObjectContext(cxt, parentNS.Client, types.NamespacedName{Name: sns.GetName()}, &corev1.Namespace{})
		if err := r.UpdateNSBasedOnParent(cxt, parentNS, ns); err != nil {
			return err
		}

		if err = r.UpdateAllNSChildrenOfNs(cxt, ns); err != nil {
			return err
		}
	}

	return nil
}

// updateMHErrorStatus updates the status of the MH object in case of an error
func (r *MigrationHierarchyReconciler) updateMHErrorStatus(upqObject *utils.ObjectContext, er error) error {
	err := upqObject.UpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.MigrationHierarchy).Status.Phase = danav1.Error
		object.(*danav1.MigrationHierarchy).Status.Reason = er.Error()
		return object, l
	})

	return err
}

// updateMHSuccessStatus updates the status of the MH object in case of a success
func (r *MigrationHierarchyReconciler) updateMHSuccessStatus(upqObject *utils.ObjectContext) error {
	err := upqObject.UpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.MigrationHierarchy).Status.Phase = danav1.Complete
		object.(*danav1.MigrationHierarchy).Status.Reason = ""
		return object, l
	})

	return err
}

// createNewSNS composes a new SNS with a Migrated Phase and then creates it
func (r *MigrationHierarchyReconciler) createSNS(sns *utils.ObjectContext, currentNamespace, toNamespace string) (*utils.ObjectContext, error) {
	snsName := sns.Object.GetName()
	resources := sns.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard
	labels := make(map[string]string)
	labels[danav1.ResourcePool] = sns.Object.GetLabels()[danav1.ResourcePool]

	composedNewSNS := ComposeSNS(currentNamespace, toNamespace, resources, labels)
	composedNewSNS.Status.Phase = danav1.Migrated

	newSNS, err := utils.NewObjectContext(sns.Ctx, r.Client, types.NamespacedName{Name: snsName}, composedNewSNS)
	if err != nil {
		return nil, err
	}

	if err := newSNS.EnsureCreateObject(); err != nil {
		return newSNS, fmt.Errorf("creating the subnamespace under the dest namespace failed because: " + err.Error())
	}

	return newSNS, nil
}

// ComposeSNS returns a subnamespace object based on the given parameters
func ComposeSNS(name string, namespace string, quota corev1.ResourceList, labels map[string]string) *danav1.Subnamespace {
	return &danav1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: danav1.SubnamespaceSpec{
			ResourceQuotaSpec: corev1.ResourceQuotaSpec{Hard: quota}},
	}
}

// deleteSNS deletes a subnamespace
func (r *MigrationHierarchyReconciler) deleteSNS(sns *utils.ObjectContext) error {
	if err := sns.EnsureDeleteObject(); err != nil {
		return fmt.Errorf("deleting the subnamespace failed because: " + err.Error())
	}

	return nil
}

// updateRole updates the role of a subnamespace
func (r *MigrationHierarchyReconciler) updateRole(sns *utils.ObjectContext, role string) error {
	err := sns.UpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger) {
		object.(*corev1.Namespace).GetLabels()[danav1.Role] = role
		return object, l
	})

	return err
}

// enqueueOriginalParent enqueues the original parent of the migrated subnamespace for reconciliation
// so that its status is updated after one or more of its children get migrated
func (r *MigrationHierarchyReconciler) enqueueOriginalParent(ctx context.Context, sourceSNSParentName string) error {
	sourceSNSParentNS, err := utils.NewObjectContext(ctx, r.Client, client.ObjectKey{Namespace: "", Name: sourceSNSParentName}, &corev1.Namespace{})
	if err != nil {
		return fmt.Errorf("failed getting namespace object '%s': "+err.Error(), sourceSNSParentName)
	}

	sourceSnsParentNamespace := utils.GetNamespaceParent(sourceSNSParentNS.Object)
	r.addSnsToSnsEvent(sourceSNSParentName, sourceSnsParentNamespace)

	return nil
}

// enqueueSNSDescendants enqueues the descendants of a subnamespace for reconciliation
func (r *MigrationHierarchyReconciler) enqueueSNSDescendants(sns *utils.ObjectContext) {
	snsDescendants := utils.GetAllChildren(sns)
	for _, sns := range snsDescendants {
		r.addSnsToSnsEvent(sns.Object.GetName(), sns.Object.GetNamespace())
	}
}

// addSnsToSnsEvent takes two parameters: snsName and snsNamespace
// then adds the sns to the sns event channel to trigger new re-sync for the sns
func (r *MigrationHierarchyReconciler) addSnsToSnsEvent(snsName string, snsNamespace string) {
	r.SnsEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snsName,
			Namespace: snsNamespace,
		},
	}}
}

// UpdateNSBasedOnParent updates the labels and annotations of a namespace
// based on its parent labels and annotations
func (r *MigrationHierarchyReconciler) UpdateNSBasedOnParent(ctx context.Context, parentNS, childNS *utils.ObjectContext) error {
	nsName := childNS.Object.GetName()
	labels, annotations := utils.GetNSLabelsAnnotationsBasedOnParent(parentNS, nsName)

	if err := childNS.AppendAnnotations(annotations); err != nil {
		return err
	}

	if err := childNS.AppendLabels(labels); err != nil {
		return err
	}

	// update the ClusterResourceQuota AnnotationSelector if needed
	isChildNSResourcePool, err := utils.IsNamespaceResourcePool(childNS)
	if err != nil {
		return err
	}

	isChildNSUpperResourcePool, err := utils.IsNSUpperResourcePool(childNS)
	if err != nil {
		return err
	}

	if !isChildNSResourcePool || isChildNSUpperResourcePool {
		if err := r.updateCRQSelector(childNS, parentNS, nsName); err != nil {
			return err
		}
	}

	// verify that the update succeeded before continuing since
	// the updates need to be serial
	if err := ensureSnsEqualAnnotations(ctx, parentNS, childNS, annotations); err != nil {
		return err
	}
	if err := ensureSnsEqualLabels(ctx, parentNS, childNS, labels); err != nil {
		return err
	}

	return nil
}

// updateCRQSelector updates the ClusterResourceQuota selector of a namespace
func (r *MigrationHierarchyReconciler) updateCRQSelector(childNS, parentNS *utils.ObjectContext, nsName string) error {
	ctx := childNS.Ctx

	crq := quotav1.ClusterResourceQuota{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: childNS.GetName()}, &crq); err != nil {
		return err
	}

	crqAnnotation := make(map[string]string)
	childNamespaceDepth := strconv.Itoa(utils.GetNamespaceDepth(parentNS.Object) + 1)

	crqAnnotation[danav1.CrqSelector+"-"+childNamespaceDepth] = nsName
	crq.Spec.Selector.AnnotationSelector = crqAnnotation
	err := r.Client.Update(ctx, &crq)

	return err
}

// ensureSnsEqualAnnotations makes sure that the annotations of a namespace are equal to the given annotations
func ensureSnsEqualAnnotations(ctx context.Context, parentNS, childNS *utils.ObjectContext, annotations map[string]string) error {
	ok := false
	retries := 0

	// To avoid an infinite loop in case of an actual failure, the loop runs at most a MAX_RETRIES number times
	for (!ok) && (retries < danav1.MaxRetries) {
		ok = true
		ns, _ := utils.NewObjectContext(ctx, parentNS.Client, types.NamespacedName{Name: childNS.GetName()}, &corev1.Namespace{})
		nsAnnotations := ns.Object.GetAnnotations()
		for key := range annotations {
			if nsAnnotations[key] != annotations[key] {
				ok = false
			}
		}
		// wait between iterations because we don't want to overload the API with many requests
		time.Sleep(danav1.SleepTimeout * time.Millisecond)
		retries++
	}
	return nil
}

// ensureSnsEqualLabels makes sure that the labels of a namespace are equal to the given labels
func ensureSnsEqualLabels(ctx context.Context, parentNS, childNS *utils.ObjectContext, labels map[string]string) error {
	ok := false
	retries := 0

	// To avoid an infinite loop in case of an actual failure, the loop runs at most a MAX_RETRIES number times
	for (!ok) && (retries < danav1.MaxRetries) {
		ok = true
		ns, _ := utils.NewObjectContext(ctx, parentNS.Client, types.NamespacedName{Name: childNS.GetName()}, &corev1.Namespace{})
		nsLabels := ns.Object.GetLabels()
		for key := range labels {
			if nsLabels[key] != labels[key] {
				ok = false
			}
		}
		// wait between iterations because we don't want to overload the API with many requests
		time.Sleep(danav1.SleepTimeout * time.Millisecond)
		retries++
	}
	return nil
}
