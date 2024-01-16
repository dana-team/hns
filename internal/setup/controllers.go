package setup

import (
	"fmt"
	. "github.com/dana-team/hns/internal/migrationhierarchy"
	. "github.com/dana-team/hns/internal/namespace"
	"github.com/dana-team/hns/internal/namespacedb"
	. "github.com/dana-team/hns/internal/rolebinding"
	. "github.com/dana-team/hns/internal/subnamespace"
	. "github.com/dana-team/hns/internal/updatequota"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	nsEvents  = make(chan event.GenericEvent)
	snsEvents = make(chan event.GenericEvent)
)

// Controllers sets up the different controllers with the manager.
func Controllers(mgr manager.Manager, ndb *namespacedb.NamespaceDB) error {
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
