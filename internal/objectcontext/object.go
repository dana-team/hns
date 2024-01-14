package objectcontext

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// CreateObject creates the objectContext.object in the cluster.
func (r *ObjectContext) CreateObject() error {
	logger := r.Log.WithName("objectContext.CreateObject")
	if err := r.Create(r.Ctx, r.Object); err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.Info(fmt.Sprintf("%s %s already exists", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
			r.present = true
			return nil
		}
		logger.Error(err, fmt.Sprintf("unable to create %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
		return err
	}
	r.present = true
	logger.Info(fmt.Sprintf("%s %s created", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
	return nil
}

// UpdateObject updates the objectContext.object in the cluster.
func (r *ObjectContext) UpdateObject(update func(object client.Object, log logr.Logger) (client.Object, logr.Logger)) error {
	logger := r.Log.WithName("objectContext.UpdateObject")
	r.Object, logger = update(r.Object, logger)
	if r.present {
		if err := r.Update(r.Ctx, r.Object); err != nil {
			if apierrors.IsConflict(err) {
				logger.Info(fmt.Sprintf("newer resource version exists for %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
				return nil
			}
			logger.Error(err, fmt.Sprintf("unable to update %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
			return err
		}
	}
	logger.Info(fmt.Sprintf("%s %s updated", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
	return nil
}

// DeleteObject deletes the objectContext.object from the cluster.
func (r *ObjectContext) DeleteObject() error {
	logger := r.Log.WithName("objectContext.DeleteObject")
	if err := r.Delete(r.Ctx, r.Object); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("%s %s does not exist", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
			r.present = false
			return nil
		}
		logger.Error(err, fmt.Sprintf("unable to delete %s %s ", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
		return err
	}
	r.present = false
	logger.Info(fmt.Sprintf("%s %s deleted", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
	return nil
}

// EnsureCreate creates the object if it doesn't exist.
func (r *ObjectContext) EnsureCreate() error {
	logger := r.Log.WithName("objectContext.EnsureCreateObject")
	if !r.IsPresent() {
		if err := r.CreateObject(); err != nil {
			return err
		}
	}

	logger.Info(fmt.Sprintf("%s %s ensured", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
	return nil
}

// EnsureDelete deletes the object if it exists.
func (r *ObjectContext) EnsureDelete() error {
	logger := r.Log.WithName("objectContext.EnsureDeleteObject")
	if r.IsPresent() {
		if err := r.DeleteObject(); err != nil {
			return err
		}
	}

	logger.Info(fmt.Sprintf("%s %s unensured", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
	return nil
}

// IsPresent checks if the objectContext.object exists the cluster.
func (r *ObjectContext) IsPresent() bool {
	return r.present
}

// forceUpdate updates the object until success or err different from conflict error.
func (r *ObjectContext) forceUpdate(ctx context.Context, update func(object client.Object, log logr.Logger) (client.Object, logr.Logger, error), isStatusUpdate bool) error {
	localLogger := r.Log.WithName("forceUpdateObject")
	var err error
	for {
		err = r.update(ctx, update, isStatusUpdate)
		if err == nil {
			return nil
		}
		if !apierrors.IsConflict(err) {
			return err
		}
		if err = r.refresh(); err != nil {
			if apierrors.IsNotFound(err) {
				localLogger.Info(fmt.Sprintf("can't update %s %s, does not exist", r.GetKindName(), r.Name()))
				return nil
			}
		}
	}
}

// update takes care of updating the object in the cluster.
func (r *ObjectContext) update(ctx context.Context, update func(object client.Object, log logr.Logger) (client.Object, logr.Logger, error), isStatusUpdate bool) error {
	localLogger := r.Log.WithName("UpdateObject")
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
				localLogger.Info(fmt.Sprintf("conflict while updating %s %s: %s", r.GetKindName(), r.Name(), err))
			} else {
				localLogger.Info(fmt.Sprintf("unable to update %s %s: %s", r.GetKindName(), r.Name(), err.Error()))
			}
			return err
		}
		localLogger.Info(fmt.Sprintf("%s %s updated", r.GetKindName(), r.Name()))

	} else {
		localLogger.Info(fmt.Sprintf("%s %s does not exists in cluster", r.GetKindName(), r.Name()))
	}
	return nil
}

// refresh takes care of refreshing an object.
func (r *ObjectContext) refresh() error {
	logger := r.Log.WithName("objectContext.RefreshObject")
	request := types.NamespacedName{Name: r.Name()}
	if r.Object.GetNamespace() != "" {
		request.Namespace = r.Object.GetNamespace()
	}

	if err := r.Get(r.Ctx, request, r.Object); err != nil {
		logger.Info(fmt.Sprintf("unable to refresh %s %s", r.Object.GetObjectKind().GroupVersionKind().Kind, r.Name()))
		return err
	}

	return nil
}

// EnsureUpdateObject force updates the object until 2 seconds timeout exceeds.
func (r *ObjectContext) EnsureUpdateObject(update func(object client.Object, log logr.Logger) (client.Object, logr.Logger, error), isStatusUpdate bool) error {
	ctx, cancel := context.WithTimeout(r.Ctx, time.Second*2)
	defer cancel()
	if err := r.forceUpdate(ctx, update, isStatusUpdate); err != nil {
		return err
	}

	return nil
}

// AppendAnnotations appends the received annotations to objectContext.object annotations.
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

// DeleteAnnotations gets a slice of strings and deletes all the annotations with keys in the slice.
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

// AppendLabels appends the received labels to objectContext.object labels.
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
