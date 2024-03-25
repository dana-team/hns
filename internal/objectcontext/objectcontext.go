package objectcontext

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

// New creates an objectContext object.
func New(ctx context.Context, Client client.Client, req types.NamespacedName, object client.Object) (*ObjectContext, error) {
	logger := log.FromContext(ctx).WithName("NewObjectContext")

	objectContext := ObjectContext{Client: Client, Object: object, Log: logger, Ctx: ctx, present: false}

	if err := Client.Get(ctx, req, object); err != nil {
		if apierrors.IsNotFound(err) {
			return &objectContext, nil
		}
		logger.Error(err, "unable to identify object")
		return nil, err
	}
	objectContext.present = true
	objectContext.Object = object

	return &objectContext, nil
}

// NewList creates a new objectContextList object.
func NewList(ctx context.Context, Client client.Client, objects client.ObjectList, req ...client.ListOption) (*ObjectContextList, error) {
	logger := log.FromContext(ctx).WithName("NewObjectContextList")

	objectContextList := ObjectContextList{Client: Client, Log: logger, Ctx: ctx, Objects: objects}

	if err := Client.List(ctx, objects, req...); err != nil {
		logger.Error(err, "unable to retriveList")
		return nil, err
	}
	objectContextList.Objects = objects

	return &objectContextList, nil
}

// Name returns the object name.
func (r *ObjectContext) Name() string {
	return r.Object.GetName()
}

// Namespace returns the object namespace.
func (r *ObjectContext) Namespace() string {
	return r.Object.GetNamespace()
}

// GetKindName returns the object kind name.
func (r *ObjectContext) GetKindName() string {
	return r.Object.GetObjectKind().GroupVersionKind().Kind
}
