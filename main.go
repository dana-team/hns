/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//shitfucktest

package main

import (
	"flag"
	"os"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/controllers"
	"github.com/dana-team/hns/internals/namespaceDB"
	"github.com/dana-team/hns/internals/server"
	"github.com/dana-team/hns/internals/webhooks"
	buildv1 "github.com/openshift/api/build/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	templatev1 "github.com/openshift/api/template/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	// +kubebuilder:scaffold:imports
)

var (
	scheme             = runtime.NewScheme()
	setupLog           = ctrl.Log.WithName("setup")
	resourcePoolEvents = make(chan event.GenericEvent)
	snsEvents          = make(chan event.GenericEvent)
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = corev1.AddToScheme(scheme)
	_ = danav1.AddToScheme(scheme)
	_ = quotav1.AddToScheme(scheme)
	_ = templatev1.AddToScheme(scheme)
	_ = buildv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func createClient() (client.Client, error) {
	scheme = runtime.NewScheme()
	cfg := ctrl.GetConfigOrDie()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = danav1.AddToScheme(scheme)
	_ = quotav1.AddToScheme(scheme)
	_ = templatev1.AddToScheme(scheme)
	_ = buildv1.AddToScheme(scheme)

	//+kubebuilder:scaffold:scheme
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	return k8sClient, err
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8081", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "c1382367.dana.hns.io",
		MetricsBindAddress: metricsAddr,
		Port:               9443,

		// uncomment here when debugging webhooks locally
		CertDir: "k8s-webhook-server/serving-certs/",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var k8sClient client.Client
	var ndb *namespaceDB.NamespaceDB
	k8sClient, err = createClient()
	ndb, err = namespaceDB.InitDB(k8sClient, setupLog.WithName("InitDB Logger"))
	if err != nil {
		setupLog.Error(err, "unable to initialize db")
		os.Exit(1)
	}

	if err = (&controllers.NamespaceReconciler{
		Client:             mgr.GetClient(),
		Log:                ctrl.Log.WithName("controllers").WithName("Namespace"),
		Scheme:             mgr.GetScheme(),
		ResourcePoolEvents: resourcePoolEvents,
		SnsEvents:          snsEvents,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Namespace")
		os.Exit(1)
	}
	if err = (&controllers.SubnamespaceReconciler{
		Client:             mgr.GetClient(),
		Log:                ctrl.Log.WithName("controllers").WithName("Subnamespace"),
		Scheme:             mgr.GetScheme(),
		ResourcePoolEvents: resourcePoolEvents,
		SnsEvents:          snsEvents,
		NamespaceDB:        ndb,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Subnamespace")
		os.Exit(1)
	}
	if err = (&controllers.RoleBindingReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Rolebinding"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Rolebinding")
		os.Exit(1)

	}
	if err = (&controllers.UpdateQuotaReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("UpdateQuota"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "UpdateQuota")
		os.Exit(1)
	}
	if err = (&controllers.MigrationHierarchyReconciler{
		Client:      mgr.GetClient(),
		Log:         ctrl.Log.WithName("controllers").WithName("MigrationHierarchy"),
		Scheme:      mgr.GetScheme(),
		NamespaceDB: ndb,
		SnsEvents:   snsEvents,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MigrationHierarchy")
		os.Exit(1)
	}

	setupLog.Info("setting up webhook server")
	hookServer := mgr.GetWebhookServer()

	decoder, _ := admission.NewDecoder(scheme)
	setupLog.Info("Registering webhooks to the webhook server")
	hookServer.Register("/validate-v1-namespace", &webhook.Admission{Handler: &webhooks.NamespaceAnnotator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
		Log:     setupLog,
	}})

	hookServer.Register("/validate-v1-subnamespace", &webhook.Admission{Handler: &webhooks.SubNamespaceAnnotator{
		Client:      mgr.GetClient(),
		Decoder:     decoder,
		Log:         setupLog,
		NamespaceDB: ndb,
	}})

	hookServer.Register("/validate-v1-rolebinding", &webhook.Admission{Handler: &webhooks.RoleBindingAnnotator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
		Log:     setupLog,
	}})

	hookServer.Register("/mutate-v1-buildconfig", &webhook.Admission{Handler: &webhooks.BuildConfigAnnotator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
		Log:     setupLog,
	}})

	hookServer.Register("/validate-v1-updatequota", &webhook.Admission{Handler: &webhooks.UpdateQuotaAnnotator{
		Client:  mgr.GetClient(),
		Decoder: decoder,
		Log:     setupLog,
	}})

	hookServer.Register("/validate-v1-migrationhierarchy", &webhook.Admission{Handler: &webhooks.MigrationHierarchyAnnotator{
		Client:      mgr.GetClient(),
		Decoder:     decoder,
		Log:         setupLog,
		NamespaceDB: ndb,
	}})

	// +kubebuilder:scaffold:builder

	ds := server.NewDiagramServer(mgr.GetClient())
	go ds.Run()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
