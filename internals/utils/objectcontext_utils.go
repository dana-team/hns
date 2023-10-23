package utils

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectContext struct {
	client.Client
	Ctx     context.Context
	Log     logr.Logger
	Object  client.Object
	present bool
}

type ObjectContextList struct {
	client.Client
	Ctx     context.Context
	Log     logr.Logger
	Objects client.ObjectList
}

// NewObjectContext Creating new objectContext object
func NewObjectContext(ctx context.Context, Client client.Client, req types.NamespacedName, object client.Object) (*ObjectContext, error) {
	logger := log.FromContext(ctx).WithName("NewObjectContext")

	objectContext := ObjectContext{Client: Client, Object: object, Log: logger, Ctx: ctx, present: false}

	if err := Client.Get(ctx, req, object); err != nil {
		if apierrors.IsNotFound(err) {
			return &objectContext, nil
		}
		logger.Error(err, fmt.Sprintf("unable to indentify object"))
		return nil, err
	}
	objectContext.present = true
	objectContext.Object = object

	return &objectContext, nil
}

// CreateObject Creating the objectContext.object in the cluster
func (r *ObjectContext) CreateObject() error {
	logger := r.Log.WithName("objectContext.CreateObject")
	if err := r.Create(r.Ctx, r.Object); err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.Info(fmt.Sprintf("%s %s already exists", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
			r.present = true
			return nil
		}
		logger.Error(err, fmt.Sprintf("unable to create %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
		return err
	}
	r.present = true
	logger.Info(fmt.Sprintf("%s %s created", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

// UpdateObject Updates the objectContext.object in the cluster
func (r *ObjectContext) UpdateObject(update func(object client.Object, log logr.Logger) (client.Object, logr.Logger)) error {
	logger := r.Log.WithName("objectContext.UpdateObject")
	r.Object, logger = update(r.Object, logger)
	if r.present {
		if err := r.Update(r.Ctx, r.Object); err != nil {
			if apierrors.IsConflict(err) {
				logger.Info(fmt.Sprintf("newer resource version exists for %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
				return nil
			}
			logger.Error(err, fmt.Sprintf("unable to update %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
			return err
		}
	}
	logger.Info(fmt.Sprintf("%s %s updated", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

// DeleteObject Deletes the objectContext.object from the cluster
func (r *ObjectContext) DeleteObject() error {
	logger := r.Log.WithName("objectContext.DeleteObject")
	if err := r.Delete(r.Ctx, r.Object); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("%s %s does not exist", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
			r.present = false
			return nil
		}
		logger.Error(err, fmt.Sprintf("unable to delete %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
		return err
	}
	r.present = false
	logger.Info(fmt.Sprintf("%s %s deleted", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

// IsPresent checks if the objectContext.object exists the cluster
func (r *ObjectContext) IsPresent() bool {
	return r.present
}

// EnsureCreateObject creates the object if it doesn't exist
func (r *ObjectContext) EnsureCreateObject() error {
	logger := r.Log.WithName("objectContext.EnsureCreateObject")
	if !r.IsPresent() {
		if err := r.CreateObject(); err != nil {
			return err
		}
	}

	logger.Info(fmt.Sprintf("%s %s ensured", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

// EnsureDeleteObject deletes the object if it exists
func (r *ObjectContext) EnsureDeleteObject() error {
	logger := r.Log.WithName("objectContext.EnsureDeleteObject")
	if r.IsPresent() {
		if err := r.DeleteObject(); err != nil {
			return err
		}
	}

	logger.Info(fmt.Sprintf("%s %s unensured", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
	return nil
}

// AppendAnnotations appends the received annotations to objectContext.object annotations
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

// DeleteAnnotations gets a slice of strings and deletes all the annotations with keys in the slice
func (r *ObjectContext) DeleteAnnotations(annotationsToDelete []string) error {
	ann := r.Object.GetAnnotations()
	for _, key := range annotationsToDelete {
		delete(ann, key)
	}
	r.Object.SetAnnotations(ann)

	if err := r.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated", "annotations")
		return object, log
	}); err != nil {
		return err
	}
	return nil
}

// AppendLabels appends the received labels to objectContext.object labels
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

// refreshObject refreshes an object
func (r *ObjectContext) refreshObject() error {
	logger := r.Log.WithName("objectContext.RefreshObject")
	request := types.NamespacedName{Name: r.Object.GetName()}
	if r.Object.GetNamespace() != "" {
		request.Namespace = r.Object.GetNamespace()
	}

	if err := r.Get(r.Ctx, request, r.Object); err != nil {
		logger.Info(fmt.Sprintf("unable to refresh %s %s", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Object.GetName()))
		return err
	}

	return nil
}

// EnsureUpdateObject force updates the object until 2 seconds timeout exceeds
func (r *ObjectContext) EnsureUpdateObject(update func(object client.Object, log logr.Logger) (client.Object, logr.Logger, error), isStatusUpdate bool) error {
	ctx, cancel := context.WithTimeout(r.Ctx, time.Second*2)
	defer cancel()
	if err := r.forceUpdateObject(ctx, update, isStatusUpdate); err != nil {
		return err
	}

	return nil
}

// forceUpdateObject updates the object until success or err different from conflict error
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

// updateObject updates the kubeject.object in the cluster
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

// GetName returns the object name
func (r *ObjectContext) GetName() string {
	return r.Object.GetName()
}

// GetKindName returns the object kind name
func (r *ObjectContext) GetKindName() string {
	return r.Object.GetObjectKind().GroupVersionKind().Kind
}

// NewObjectContextList creates a new objectContextList object
func NewObjectContextList(ctx context.Context, Client client.Client, objects client.ObjectList, req ...client.ListOption) (*ObjectContextList, error) {
	logger := log.FromContext(ctx).WithName("NewObjectContextList")

	//Creating the objectContextList
	objectContextList := ObjectContextList{Client: Client, Log: logger, Ctx: ctx, Objects: objects}

	//Creating the objectContextList.objects
	if err := Client.List(ctx, objects, req...); err != nil {
		logger.Error(err, fmt.Sprintf("unable to retriveList"))
		return nil, err
	}
	objectContextList.Objects = objects

	return &objectContextList, nil
}
