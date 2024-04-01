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

package main

import (
	"flag"
	userv1 "github.com/openshift/api/user/v1"
	"os"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/namespacedb"
	"github.com/dana-team/hns/internal/setup"
	buildv1 "github.com/openshift/api/build/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	templatev1 "github.com/openshift/api/template/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

var (
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	noWebhooks           bool
	onlyResourcePool     bool
	maxSNS               int
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(danav1.AddToScheme(scheme))
	utilruntime.Must(quotav1.Install(scheme))
	utilruntime.Must(templatev1.Install(scheme))
	utilruntime.Must(buildv1.Install(scheme))
	utilruntime.Must(userv1.Install(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	parseFlags()

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "c1382367.dana.io",
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var ndb *namespacedb.NamespaceDB
	ndb, err = namespacedb.Init(scheme, setupLog.WithName("InitDB Logger"))
	if err != nil {
		setupLog.Error(err, "unable to successfully initialize namespacedb")
		os.Exit(1)
	}

	hnsOpts := setup.Options{
		NoWebhooks:        noWebhooks,
		OnlyResourcePool:  onlyResourcePool,
		MaxSNSInHierarchy: maxSNS,
	}

	setupLog.Info("setting up reconcilers")
	if err := setup.Controllers(mgr, ndb); err != nil {
		setupLog.Error(err, "unable to successfully set up controllers")
		os.Exit(1)
	}

	if !hnsOpts.NoWebhooks {
		setupLog.Info("setting up webhooks")
		setup.Webhooks(mgr, ndb, scheme, hnsOpts)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func parseFlags() {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&noWebhooks, "no-webhooks", false, "Disables webhooks")
	flag.BoolVar(&onlyResourcePool, "only-resourcepool", false, "Only allow creation of resourcepools")
	flag.IntVar(&maxSNS, "max-sns", 250, "The maximum number of subnamespaces under a single CRQ")

	flag.Parse()
}
