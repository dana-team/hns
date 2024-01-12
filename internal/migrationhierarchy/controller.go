package migrationhierarchy

import (
	"context"
	"fmt"
	"github.com/dana-team/hns/internal/common"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/namespacedb"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/quota"
	"github.com/dana-team/hns/internal/subnamespace/snsutils"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	danav1 "github.com/dana-team/hns/api/v1"
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
	NamespaceDB *namespacedb.NamespaceDB
	SnsEvents   chan event.GenericEvent
}

// +kubebuilder:rbac:groups=dana.hns.io,resources=migrationhierarchies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dana.hns.io,resources=migrationhierarchies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=users,verbs=impersonate

func (r *MigrationHierarchyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&danav1.MigrationHierarchy{}).
		Complete(r)
}

func (r *MigrationHierarchyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("controllers").WithName("MigrationHierarchy").WithValues("mh", req.NamespacedName)
	logger.Info("starting to reconcile")

	mhObject, err := objectcontext.New(ctx, r.Client, client.ObjectKey{Name: req.NamespacedName.Name}, &danav1.MigrationHierarchy{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get object %q: "+err.Error(), mhObject.Name())
	}

	if !mhObject.IsPresent() {
		logger.Info("resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	phase := mhObject.Object.(*danav1.MigrationHierarchy).Status.Phase
	if common.ShouldReconcile(phase) {
		return r.reconcile(mhObject)
	} else {
		logger.Info("no need to reconcile, object phase is: ", "phase", phase)
	}

	return ctrl.Result{}, nil
}

func (r *MigrationHierarchyReconciler) reconcile(mhObject *objectcontext.ObjectContext) (ctrl.Result, error) {
	ctx := mhObject.Ctx
	logger := log.FromContext(ctx)
	phase := mhObject.Object.(*danav1.MigrationHierarchy).Status.Phase

	currentNamespace := mhObject.Object.(*danav1.MigrationHierarchy).Spec.CurrentNamespace
	toNamespace := mhObject.Object.(*danav1.MigrationHierarchy).Spec.ToNamespace

	ns, er := objectcontext.New(ctx, r.Client, client.ObjectKey{Name: currentNamespace}, &corev1.Namespace{})
	if er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed getting namespace object %q: "+er.Error(), currentNamespace)
	}

	sourceSNSParentName := nsutils.Parent(ns.Object)
	oldSNS, er := objectcontext.New(ctx, r.Client, client.ObjectKey{Name: currentNamespace, Namespace: sourceSNSParentName}, &danav1.Subnamespace{})
	if er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed getting subnamespace object %q: "+er.Error(), currentNamespace)
	}

	sourceQuotaObj, err := quota.NamespaceObject(ns)
	if er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed getting quota object %q: "+er.Error(), currentNamespace)
	}

	sourceResources := quota.GetQuotaObjectSpec(sourceQuotaObj.Object)
	rootNSName := nsutils.DisplayNameSlice(ns)[0]

	// add the resources that are allocated to the migrated subnamespace to the new parent using UpdateQuota API
	sourceQuotaObjExists, _, _ := quota.DoesSubnamespaceObjectExist(oldSNS)

	toNS, er := objectcontext.New(ctx, r.Client, client.ObjectKey{Name: toNamespace}, &corev1.Namespace{})
	if er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed to get namespace %q: "+er.Error(), toNamespace)
	}

	toSNSParentName := nsutils.Parent(toNS.Object)
	toSNS, er := objectcontext.New(ctx, r.Client, client.ObjectKey{Name: toNamespace, Namespace: toSNSParentName}, &danav1.Subnamespace{})
	if er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed getting subnamespace object %q: "+er.Error(), toNamespace)
	}

	if phase == danav1.None {
		if sourceQuotaObjExists {
			// temporarily increase the quota of the root namespace to make
			// sure the migrationUPQ doesn't result in an error due to insufficient resources
			if er := increaseRootResources(mhObject, rootNSName, sourceResources); err != nil {
				err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
				}
				return ctrl.Result{}, fmt.Errorf("failed increasing root resources for migration %q: "+er.Error(), mhObject.Name())
			}

			toSNSCRQPointer := toSNS.Object.(*danav1.Subnamespace).Annotations[danav1.CrqPointer]
			if er := createMigrationUPQ(mhObject, sourceResources, rootNSName, toSNSCRQPointer); err != nil {
				err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
				}
				return ctrl.Result{}, fmt.Errorf("failed create updateQuota for migration %q: "+er.Error(), mhObject.Name())
			}
		}

		// update the phase of the Migration Hierarchy to make sure that in case of an error or a requeue, the resources
		// that have been added for the migration will not be added again
		if err := r.updateMHStatus(mhObject, danav1.InProgress, ""); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		logger.Info("successfully updated status of MigrationHierarchy object", "phase", danav1.InProgress)
	}

	// requeue until the updateQuota is Complete to make sure resources are updated
	// before moving forward with the migration process
	if sourceQuotaObjExists {
		if res, er := monitorMigrationUPQ(mhObject, rootNSName); er != nil {
			err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
			}
			return ctrl.Result{}, fmt.Errorf("failed to add migration resources: " + er.Error())
		} else if !res.IsZero() {
			return res, nil
		}
		logger.Info("successfully added resources for migration", "new parent", toNamespace)
	}

	// resources have been added to the new parent to complete the migration, so continue with migration
	newSNS, er := r.createNewSNS(oldSNS, mhObject, currentNamespace, toNamespace)
	if er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed creating subnamespace %q under namespace '%s': "+er.Error(), currentNamespace, toNamespace)
	}

	logger.Info("successfully created new subnamespace under new parent", "subnamespace", currentNamespace, "new parent", toNamespace)

	if er := r.deleteOldSNS(oldSNS, newSNS, mhObject); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed deleting old subnamespace '%s' :"+er.Error(), oldSNS.Name())
	}

	logger.Info("successfully deleted old subnamespace from old parent", "subnamesapce", currentNamespace, "old parent", sourceSNSParentName)

	if er := r.updateRelatedObjects(mhObject, toNS, ns); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed to update related objects: " + er.Error())
	}
	logger.Info("successfully updated related objects of subnamespace", "subnamespace", currentNamespace)

	// after the migration is completed, we need to update the db to account for the new parent
	// MigrateNsHierarchy updates the namespace and its children hierarchy to be under the new parent in the DB
	if er := namespacedb.MigrateNSHierarchy(ctx, r.NamespaceDB, r.Client, ns.Name(), toNS.Name()); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed migrating subnamespace '%s' in namespacedb: "+er.Error(), ns.Name())
	}
	logger.Info("successfully migrated subnamespace in namespacedb", "subnamespace", currentNamespace)

	// enqueue for reconciliation the original parent of the subnamespace that should be migrated in order for
	// the old parent's status to show the now-changed list of child subnamespaces
	if er := r.enqueueOriginalParent(ctx, sourceSNSParentName); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed to enqueue '%s': "+er.Error(), sourceSNSParentName)
	}
	logger.Info("successfully enqueued original parent namespace", "oldParent", sourceSNSParentName)

	// enqueue for reconciliation the descendants of the subnamespace so that their labels and annotations
	// are updated properly
	r.enqueueSNSDescendants(newSNS)
	logger.Info("successfully enqueued descendants of subnamespace", "subnamespace", currentNamespace)

	// subtract the resources that were allocated to the migrated subnamespace from the old parent using UpdateQuota API,
	// don't wait for it to finish since it shouldn't block
	if sourceQuotaObjExists {
		if er := createMigrationUPQ(mhObject, sourceResources, sourceSNSParentName, rootNSName); er != nil {
			err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
			}
			return ctrl.Result{}, fmt.Errorf("failed to create updateQuota for migration '%s': "+er.Error(), mhObject.Name())
		}
		// decrease the quota of the root namespace which was temporarily added
		if er := decreaseRootResources(mhObject, rootNSName, sourceResources); er != nil {
			err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
			}
			return ctrl.Result{}, fmt.Errorf("failed decreasing root resources for migration")
		}
	}

	if er := r.updateMHStatus(mhObject, danav1.Complete, ""); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
		}
		return ctrl.Result{}, fmt.Errorf("failed updating the status of object '%s': "+er.Error(), mhObject.Name())
	}
	logger.Info("successfully updated status of MigrationHierarchy object", "phase", danav1.Complete)

	return ctrl.Result{}, nil
}

// createNewSNS handles the creation of the migrated subnamespace under a new parent.
func (r *MigrationHierarchyReconciler) createNewSNS(sns, mhObject *objectcontext.ObjectContext, currentNamespace, toNamespace string) (*objectcontext.ObjectContext, error) {
	newSNS, er := r.createSNS(sns, currentNamespace, toNamespace)
	if er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return nil, fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
		}
		return nil, er
	}

	return newSNS, nil
}

// deleteOldSNS handles the deleting the migrated subnamespace.
func (r *MigrationHierarchyReconciler) deleteOldSNS(oldSNS, newSNS, mhObject *objectcontext.ObjectContext) error {
	if er := r.deleteSNS(oldSNS); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
		}

		// if deleting the old subnamespace fails, delete the newly created subnamespace
		if er := r.deleteSNS(newSNS); er != nil {
			err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
			if err != nil {
				return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.Name())
			}
			return fmt.Errorf("failed deleting subnamespace '%s' :"+err.Error(), newSNS.Name())
		}
	}

	return nil
}

// createNewSNS composes a new SNS with a Migrated Phase and then creates it.
func (r *MigrationHierarchyReconciler) createSNS(sns *objectcontext.ObjectContext, currentNamespace, toNamespace string) (*objectcontext.ObjectContext, error) {
	snsName := sns.Name()
	resources := sns.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard
	labels := make(map[string]string)
	labels[danav1.ResourcePool] = sns.Object.GetLabels()[danav1.ResourcePool]

	composedNewSNS := ComposeSNS(currentNamespace, toNamespace, resources, labels)
	composedNewSNS.Status.Phase = danav1.Migrated

	newSNS, err := objectcontext.New(sns.Ctx, r.Client, types.NamespacedName{Name: snsName}, composedNewSNS)
	if err != nil {
		return nil, err
	}

	if err := newSNS.EnsureCreate(); err != nil {
		return newSNS, fmt.Errorf("creating the subnamespace under the destination namespace failed because: " + err.Error())
	}

	return newSNS, nil
}

// ComposeSNS returns a subnamespace object based on the given parameters.
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
func (r *MigrationHierarchyReconciler) deleteSNS(sns *objectcontext.ObjectContext) error {
	if err := sns.EnsureDelete(); err != nil {
		return fmt.Errorf("deleting the subnamespace failed because: " + err.Error())
	}

	return nil
}

// enqueueOriginalParent enqueues the original parent of the migrated subnamespace for reconciliation
// so that its status is updated after one or more of its children get migrated.
func (r *MigrationHierarchyReconciler) enqueueOriginalParent(ctx context.Context, sourceSNSParentName string) error {
	sourceSNSParentNS, err := objectcontext.New(ctx, r.Client, client.ObjectKey{Name: sourceSNSParentName}, &corev1.Namespace{})
	if err != nil {
		return fmt.Errorf("failed getting namespace object '%s': "+err.Error(), sourceSNSParentName)
	}

	sourceSnsParentNamespace := nsutils.Parent(sourceSNSParentNS.Object)
	r.addSnsToSnsEvent(sourceSNSParentName, sourceSnsParentNamespace)

	return nil
}

// enqueueSNSDescendants enqueues the descendants of a subnamespace for reconciliation.
func (r *MigrationHierarchyReconciler) enqueueSNSDescendants(sns *objectcontext.ObjectContext) {
	snsDescendants := snsutils.GetAllChildren(sns)
	for _, sns := range snsDescendants {
		r.addSnsToSnsEvent(sns.Name(), sns.Namespace())
	}
}

// addSnsToSnsEvent takes two parameters: snsName and snsNamespace
// then adds the sns to the sns event channel to trigger new re-sync for the sns.
func (r *MigrationHierarchyReconciler) addSnsToSnsEvent(snsName string, snsNamespace string) {
	r.SnsEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snsName,
			Namespace: snsNamespace,
		},
	}}
}

// ensureSnsEqualAnnotations makes sure that the annotations of a namespace are equal to the given annotations.
func ensureSnsEqualAnnotations(ctx context.Context, parentNS, childNS *objectcontext.ObjectContext, annotations map[string]string) error {
	ok := false
	retries := 0

	// To avoid an infinite loop in case of an actual failure, the loop runs at most a MAX_RETRIES number times
	for (!ok) && (retries < danav1.MaxRetries) {
		ok = true
		ns, _ := objectcontext.New(ctx, parentNS.Client, types.NamespacedName{Name: childNS.Name()}, &corev1.Namespace{})
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

// ensureSnsEqualLabels makes sure that the labels of a namespace are equal to the given labels.
func ensureSnsEqualLabels(ctx context.Context, parentNS, childNS *objectcontext.ObjectContext, labels map[string]string) error {
	ok := false
	retries := 0

	// To avoid an infinite loop in case of an actual failure, the loop runs at most a MAX_RETRIES number times
	for (!ok) && (retries < danav1.MaxRetries) {
		ok = true
		ns, _ := objectcontext.New(ctx, parentNS.Client, types.NamespacedName{Name: childNS.Name()}, &corev1.Namespace{})
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

// updateMHStatus updates the status of the MH object.
func (r *MigrationHierarchyReconciler) updateMHStatus(upqObject *objectcontext.ObjectContext, phase danav1.Phase, reason string) error {
	err := upqObject.UpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.MigrationHierarchy).Status.Phase = phase
		object.(*danav1.MigrationHierarchy).Status.Reason = reason
		return object, l
	})

	return err
}
