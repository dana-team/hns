package webhooks

import (
	. "github.com/dana-team/hns/internals/webhooks/mutate/buildconfig"
	. "github.com/dana-team/hns/internals/webhooks/validate/migrationhierarchy"
	. "github.com/dana-team/hns/internals/webhooks/validate/namespace"
	. "github.com/dana-team/hns/internals/webhooks/validate/rolebinding"
	. "github.com/dana-team/hns/internals/webhooks/validate/subnamespace"
	. "github.com/dana-team/hns/internals/webhooks/validate/updatequota"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/dana-team/hns/internals/namespaceDB"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhooks registers the different webhooks
func SetupWebhooks(mgr manager.Manager, ndb *namespaceDB.NamespaceDB, scheme *runtime.Scheme) {
	hookServer := mgr.GetWebhookServer()

	decoder, _ := admission.NewDecoder(scheme)
	hookServer.Register("/validate-v1-namespace", &webhook.Admission{Handler: &NamespaceAnnotator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})

	hookServer.Register("/validate-v1-subnamespace", &webhook.Admission{Handler: &SubnamespaceAnnotator{
		Client:      mgr.GetClient(),
		Decoder:     decoder,
		NamespaceDB: ndb,
	}})

	hookServer.Register("/validate-v1-rolebinding", &webhook.Admission{Handler: &RoleBindingAnnotator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})

	hookServer.Register("/mutate-v1-buildconfig", &webhook.Admission{Handler: &BuildConfigAnnotator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})

	hookServer.Register("/validate-v1-updatequota", &webhook.Admission{Handler: &UpdateQuotaAnnotator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})

	hookServer.Register("/validate-v1-migrationhierarchy", &webhook.Admission{Handler: &MigrationHierarchyAnnotator{
		Client:      mgr.GetClient(),
		Decoder:     decoder,
		NamespaceDB: ndb,
	}})
}
