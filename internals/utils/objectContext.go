package utils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"time"
)

type ObjectContext struct {
	client.Client
	Ctx     context.Context
	Log     logr.Logger
	Object  client.Object
	present bool
}

//NewObjectContext Creating new objectContext object
func NewObjectContext(ctx context.Context, Log logr.Logger, Client client.Client, req types.NamespacedName, object client.Object) (*ObjectContext, error) {
	log := Log.WithName("NewObjectContext")

	//Creating the objectContext
	objectContext := ObjectContext{Client: Client, Object: object, Log: Log, Ctx: ctx, present: false}

	//Update objectContext.object if he exists in the cluster
	if err := Client.Get(ctx, req, object); err != nil {
		if apierrors.IsNotFound(err) {
			return &objectContext, nil
		}
		log.Error(err, fmt.Sprintf("unable to indentify object"))
		return nil, err
	}
	objectContext.present = true
	objectContext.Object = object

	return &objectContext, nil
}

//CreateObject Creating the objectContext.object in the cluster
func (r *ObjectContext) CreateObject() error {
	log := r.Log.WithName("objectContext.CreateObject")
	if err := r.Create(r.Ctx, r.Object); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info(fmt.Sprintf("%s %s already exists", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
			r.present = true
			return nil
		}
		log.Error(err, fmt.Sprintf("unable to create %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
		return err
	}
	r.present = true
	log.Info(fmt.Sprintf("%s %s created", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

//UpdateObject Updates the objectContext.object in the cluster
func (r *ObjectContext) UpdateObject(update func(object client.Object, log logr.Logger) (client.Object, logr.Logger)) error {
	log := r.Log.WithName("objectContext.UpdateObject")
	r.Object, log = update(r.Object, log)
	if r.present {
		if err := r.Update(r.Ctx, r.Object); err != nil {
			if apierrors.IsConflict(err) {
				log.Info(fmt.Sprintf("newer resource version exists for %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
				return nil
			}
			log.Error(err, fmt.Sprintf("unable to update %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
			return err
		}
	}
	log.Info(fmt.Sprintf("%s %s updated", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

//DeleteObject Deletes the objectContext.object from the cluster
func (r *ObjectContext) DeleteObject() error {
	log := r.Log.WithName("objectContext.DeleteObject")
	if err := r.Delete(r.Ctx, r.Object); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info(fmt.Sprintf("%s %s does not exist", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
			r.present = false
			return nil
		}
		log.Error(err, fmt.Sprintf("unable to delete %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
		return err
	}
	r.present = false
	log.Info(fmt.Sprintf("%s %s deleted", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

//IsPresent checks if the objectContext.object exists the cluster
func (r *ObjectContext) IsPresent() bool {
	return r.present
}

// EnsureCreateObject creates the object if it doesn't exist
func (r *ObjectContext) EnsureCreateObject() error {
	log := r.Log.WithName("objectContext.EnsureCreateObject")
	if !r.IsPresent() {
		if err := r.CreateObject(); err != nil {
			return err
		}
	}

	log.Info(fmt.Sprintf("%s %s ensured", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

// EnsureDeleteObject deletes the object if it exist
func (r *ObjectContext) EnsureDeleteObject() error {
	log := r.Log.WithName("objectContext.EnsureDeleteObject")
	if r.IsPresent() {
		if err := r.DeleteObject(); err != nil {
			return err
		}
	}

	log.Info(fmt.Sprintf("%s %s unensured", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

//AppendAnnotations appends the received annotations to objectContext.object annotations
func (r *ObjectContext) AppendAnnotations(annotationsToAppend map[string]string) error {
	newAnnotations := r.Object.GetAnnotations()
	if newAnnotations == nil {
		newAnnotations = annotationsToAppend
	} else {
		for key, value := range annotationsToAppend {
			newAnnotations[key] = value
		}
	}
	r.Object.SetAnnotations(newAnnotations)

	if err := r.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated", "annotations")
		return object, log
	}); err != nil {
		return err
	}
	return nil
}

//AppendLabels appends the received labels to objectContext.object labels
func (r *ObjectContext) AppendLabels(labelsToAppend map[string]string) error {
	newLabels := r.Object.GetLabels()

	for key, value := range labelsToAppend {
		newLabels[key] = value
	}
	r.Object.SetLabels(newLabels)

	if err := r.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated", "labels")
		return object, log
	}); err != nil {
		return err
	}
	return nil
}

func (r *ObjectContext) refreshObject() error {
	log := r.Log.WithName("objectContext.RefreshObject")
	request := types.NamespacedName{Name: r.Object.GetName()}
	if r.Object.GetNamespace() != "" {
		request.Namespace = r.Object.GetNamespace()
	}

	if err := r.Get(r.Ctx, request, r.Object); err != nil {
		log.Info(fmt.Sprintf("unable to refresh %s %s", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
		return err
	}

	return nil
}

func (r *ObjectContext) UpdateNsByparent(nsparent *ObjectContext, nsChild *ObjectContext) error {
	nsChild.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		oldcrq := quotav1.ClusterResourceQuota{}
		r.Client.Get(context.TODO(), types.NamespacedName{
			Name: nsChild.GetName(),
		}, &oldcrq)
		childNamespaceDepth := strconv.Itoa(GetNamespaceDepth(nsparent.Object) + 1)
		parentDisplayName := GetNamespaceDisplayName(nsparent.Object)
		nsName := nsChild.Object.GetName()

		//update ns annotations
		ann := getParentAnnotations(nsparent)
		ann[danav1.Depth] = childNamespaceDepth
		ann[danav1.DisplayName] = parentDisplayName + "/" + nsName
		ann[danav1.SnsPointer] = nsName
		ann[danav1.CrqSelector+"-"+childNamespaceDepth] = nsName
		object.(*corev1.Namespace).SetAnnotations(ann)

		//update crq AnnotationSelector
		crqAnn := make(map[string]string)
		crqAnn[danav1.CrqSelector+"-"+childNamespaceDepth] = nsName
		oldcrq.Spec.Selector.AnnotationSelector = crqAnn
		r.Client.Update(context.TODO(), &oldcrq)

		//update ns labels
		labels := getParentAggragators(nsparent)
		labels[danav1.Aggragator+nsName] = "true"
		labels[danav1.Parent] = nsparent.Object.(*corev1.Namespace).Name
		labels[danav1.Hns] = "true"
		labels[danav1.ResourcePool] = GetNamespaceResourcePooled(nsChild)
		object.(*corev1.Namespace).SetLabels(labels)

		log = log.WithValues("update ns labels and annotations", nsName)
		return object, log
	})
	return nil
}

func getParentAnnotations(copyFrom *ObjectContext) map[string]string {
	parentAnnotations := map[string]string{}

	ContainsAny := func(key string, annotations []string) bool {
		for _, annotation := range annotations {
			if strings.Contains(key, annotation) {
				return true
			}
		}
		return false
	}

	for key, value := range copyFrom.Object.GetAnnotations() {
		if ContainsAny(key, danav1.DefaultAnnotations) {
			parentAnnotations[key] = value
		}
	}
	return parentAnnotations
}

func getParentAggragators(copyFrom *ObjectContext) map[string]string {
	parentAggragators := map[string]string{}
	for key, value := range copyFrom.Object.GetLabels() {
		if strings.Contains(key, danav1.Aggragator) {
			parentAggragators[key] = value
		}
	}
	return parentAggragators
}

//EnsureUpdateObject force updates the object until 2 seconds timeout exceeds
func (r *ObjectContext) EnsureUpdateObject(update func(object client.Object, log logr.Logger) (client.Object, logr.Logger, error), isStatusUpdate bool) error {
	ctx, cancel := context.WithTimeout(r.Ctx, time.Second*2)
	defer cancel()
	if err := r.forceUpdateObject(ctx, update, isStatusUpdate); err != nil {
		return err
	}

	return nil
}

//forceUpdateObject updates the object until success or err different than conflict error
func (r *ObjectContext) forceUpdateObject(ctx context.Context, update func(object client.Object, log logr.Logger) (client.Object, logr.Logger, error), isStatusUpdate bool) error {
	localLogger := r.Log.WithName("kubeject.forceUpdateObject")
	var err error
	for {
		err = r.updateObject(ctx, update, isStatusUpdate)
		if err == nil {
			return nil
		}
		if !apierrors.IsConflict(err) {
			return err
		}
		if err = r.refreshObject(); err != nil {
			if apierrors.IsNotFound(err) {
				localLogger.Info(fmt.Sprintf("can't update %s %s, does not exist", r.GetKindName(), r.GetName()))
				return nil
			}
		}
	}
}

//updateObject updates the kubeject.object in the cluster
func (r *ObjectContext) updateObject(ctx context.Context, update func(object client.Object, log logr.Logger) (client.Object, logr.Logger, error), isStatusUpdate bool) error {
	localLogger := r.Log.WithName("kubeject.UpdateObject")
	if r.present {
		var err error
		r.Object, localLogger, err = update(r.Object, localLogger)
		if err != nil {
			return err
		}
		if isStatusUpdate {
			err = r.Status().Update(ctx, r.Object)
		} else {
			err = r.Update(ctx, r.Object)
		}
		if err != nil {
			if apierrors.IsConflict(err) {
				localLogger.Info(fmt.Sprintf("conflict while updating %s %s: %s", r.GetKindName(), r.GetName(), err))
			} else {
				localLogger.Info(fmt.Sprintf("unable to update %s %s: %s", r.GetKindName(), r.GetName(), err.Error()))
			}
			return err
		}
		localLogger.Info(fmt.Sprintf("%s %s updated", r.GetKindName(), r.GetName()))

	} else {
		localLogger.Info(fmt.Sprintf("%s %s does not exists in cluster", r.GetKindName(), r.GetName()))
	}
	return nil
}

//GetName returns the object name
func (r *ObjectContext) GetName() string {
	return r.Object.GetName()
}

//GetKindName returns the object kind name
func (r *ObjectContext) GetKindName() string {
	return r.Object.GetObjectKind().GroupVersionKind().Kind
}
