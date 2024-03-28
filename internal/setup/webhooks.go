package setup

import (
	. "github.com/dana-team/hns/internal/buildconfig"
	. "github.com/dana-team/hns/internal/migrationhierarchy"
	. "github.com/dana-team/hns/internal/namespace"
	"github.com/dana-team/hns/internal/namespacedb"
	. "github.com/dana-team/hns/internal/rolebinding"
	. "github.com/dana-team/hns/internal/subnamespace"
	. "github.com/dana-team/hns/internal/updatequota"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Options struct {
	NoWebhooks        bool
	OnlyResourcePool  bool
	MaxSNSInHierarchy int
}

// Webhooks registers the different webhooks.
func Webhooks(mgr manager.Manager, ndb *namespacedb.NamespaceDB, scheme *runtime.Scheme, opts Options) {
	hookServer := mgr.GetWebhookServer()

	decoder := admission.NewDecoder(scheme)

	hookServer.Register("/validate-v1-namespace", &webhook.Admission{Handler: &NamespaceValidator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})

	hookServer.Register("/validate-v1-subnamespace", &webhook.Admission{Handler: &SubnamespaceValidator{
		Client:      mgr.GetClient(),
		Decoder:     decoder,
		NamespaceDB: ndb,
		MaxSNS:      opts.MaxSNSInHierarchy,
		OnlyRP:      opts.OnlyResourcePool,
	}})

	hookServer.Register("/validate-v1-rolebinding", &webhook.Admission{Handler: &RoleBindingValidator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})

	hookServer.Register("/mutate-v1-buildconfig", &webhook.Admission{Handler: &BuildConfigMutator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})
	hookServer.Register("/mutate-v1-migrationhierarchy", &webhook.Admission{Handler: &MigrationHierarchyMutator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})
	hookServer.Register("/mutate-v1-updatequota", &webhook.Admission{Handler: &UpdateQuotaMutator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})

	hookServer.Register("/validate-v1-updatequota", &webhook.Admission{Handler: &UpdateQuotaValidator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
	}})

	hookServer.Register("/validate-v1-migrationhierarchy", &webhook.Admission{Handler: &MigrationHierarchyValidator{
		Client:      mgr.GetClient(),
		Decoder:     decoder,
		NamespaceDB: ndb,
		MaxSNS:      opts.MaxSNSInHierarchy,
	}})
}
