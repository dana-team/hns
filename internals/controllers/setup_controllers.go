package controllers

import (
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/dana-team/hns/internals/controllers/migrationhierarchy"
	. "github.com/dana-team/hns/internals/controllers/namespace"
	. "github.com/dana-team/hns/internals/controllers/rolebinding"
	. "github.com/dana-team/hns/internals/controllers/subnamespace"
	. "github.com/dana-team/hns/internals/controllers/updatequota"
	"github.com/dana-team/hns/internals/namespaceDB"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var (
	nsEvents  = make(chan event.GenericEvent)
	snsEvents = make(chan event.GenericEvent)
)

// SetupControllers sets up the different controllers with the manager
func SetupControllers(mgr manager.Manager, ndb *namespaceDB.NamespaceDB) error {
	if err := (&NamespaceReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		NSEvents:    nsEvents,
		SNSEvents:   snsEvents,
		NamespaceDB: ndb,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller: "+err.Error(), "controller", "Namespace")
	}

	if err := (&SubnamespaceReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		NSEvents:    nsEvents,
		SNSEvents:   snsEvents,
		NamespaceDB: ndb,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller: "+err.Error(), "controller", "Subnamespace")
	}

	if err := (&RoleBindingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller: "+err.Error(), "controller", "RoleBinding")
	}

	if err := (&UpdateQuotaReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller: "+err.Error(), "controller", "UpdateQuota")
	}

	if err := (&MigrationHierarchyReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		NamespaceDB: ndb,
		SnsEvents:   snsEvents,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller: "+err.Error(), "controller", "MigrationHierarchy")
	}

	return nil
}
