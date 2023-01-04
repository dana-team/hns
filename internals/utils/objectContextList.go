package utils

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectContextList struct {
	client.Client
	Ctx     context.Context
	Log     logr.Logger
	Objects client.ObjectList
}

//NewObjectContext Creating new objectContext object
func NewObjectContextList(ctx context.Context, Log logr.Logger, Client client.Client, objects client.ObjectList, req ...client.ListOption) (*ObjectContextList, error) {
	log := Log.WithName("NewObjectContextList")

	//Creating the objectContextList
	objectContextList := ObjectContextList{Client: Client, Log: Log.WithName("objectContextList"), Ctx: ctx, Objects: objects}

	//Creating the objectContextList.objects
	if err := Client.List(ctx, objects, req...); err != nil {
		log.Error(err, fmt.Sprintf("unable to retriveList"))
		return nil, err
	}
	objectContextList.Objects = objects

	return &objectContextList, nil
}
