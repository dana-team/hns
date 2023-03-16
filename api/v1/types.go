package v1

import (
	"os"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Phase string

// Sns phases
const (
	Missing  Phase = "Missing"
	Created  Phase = "Created"
	None     Phase = ""
	Migrated Phase = "Migrated"
	Complete Phase = "Complete"
	Error    Phase = "Error"
)

const (
	Root   string = "root"
	NoRole string = "none"
	Leaf   string = "leaf"
	True   string = "True"
	False  string = "False"
)

// MetaGroup
const MetaGroup = "dana.hns.io/"

// Labels
const (
	Hns          = MetaGroup + "subnamespace"
	Parent       = MetaGroup + "parent"
	Aggragator   = MetaGroup + "aggragator-"
	ResourcePool = MetaGroup + "resourcepool"
)

// Annotations
const (
	Role            = MetaGroup + "role"
	Depth           = MetaGroup + "depth"
	CrqSelector     = MetaGroup + "crq-selector"
	RootCrqSelector = CrqSelector + "-0"
	Pointer         = MetaGroup + "pointer"
	SnsPointer      = MetaGroup + "sns-pointer"
	DisplayName     = "openshift.io/display-name"
	RqDepth         = MetaGroup + "rq-depth"
	IsRq            = MetaGroup + "is-rq"
	IsSecondaryRoot = MetaGroup + "is-secondary-root"
	IsUpperRp       = MetaGroup + "is-upper-rp"
	UpperRp         = MetaGroup + "upper-rp"
	CrqPointer      = MetaGroup + "crq-pointer"
)

// Finalizers
const (
	NsFinalizer = MetaGroup + "delete-sns"
	RbFinalizer = MetaGroup + "delete-rb"
)

// IsRq offsets
// Secondary Roots
const (
	SelfOffset   = 0
	ParentOffset = -1
	ChildOffset  = 1
)

var (
	DefaultAnnotations = []string{CrqSelector, "scheduler.alpha.kubernetes.io/defaultTolerations", "openshift.io/node-selector"}
	MaxSNS, _          = strconv.Atoi(os.Getenv("MAX_SNS_IN_HIERARCHY"))
)

var (
	Configmaps             = resource.NewQuantity(100, resource.DecimalSI)
	Builds                 = resource.NewQuantity(100, resource.DecimalSI)
	Cronjobs               = resource.NewQuantity(100, resource.DecimalSI)
	Daemonsets             = resource.NewQuantity(100, resource.DecimalSI)
	Deployments            = resource.NewQuantity(100, resource.DecimalSI)
	Jobs                   = resource.NewQuantity(100, resource.DecimalSI)
	Replicasets            = resource.NewQuantity(100, resource.DecimalSI)
	Routes                 = resource.NewQuantity(100, resource.DecimalSI)
	Pvc                    = resource.NewQuantity(100, resource.DecimalSI)
	Replicationcontrollers = resource.NewQuantity(100, resource.DecimalSI)
	Secrets                = resource.NewQuantity(100, resource.DecimalSI)
	deploymentconfigs      = resource.NewQuantity(100, resource.DecimalSI)
	buildconfigs           = resource.NewQuantity(100, resource.DecimalSI)
	serviceaccounts        = resource.NewQuantity(100, resource.DecimalSI)
	statefulsets           = resource.NewQuantity(100, resource.DecimalSI)
	templates              = resource.NewQuantity(100, resource.DecimalSI)
	imagestreams           = resource.NewQuantity(100, resource.DecimalSI)
	ZeroDecimal            = resource.NewQuantity(0, resource.DecimalSI)

	Quotahard = v1.ResourceList{"configmaps": *Configmaps, "count/builds.build.openshift.io": *Builds, "count/cronjobs.batch": *Cronjobs, "count/daemonsets.apps": *Daemonsets,
		"count/deployments.apps": *Deployments, "count/jobs.batch": *Cronjobs, "count/replicasets.apps": *Replicasets, "count/routes.route.openshift.io": *Routes,
		"secrets": *Secrets, "count/deploymentconfigs.apps.openshift.io": *deploymentconfigs, "count/buildconfigs.build.openshift.io": *buildconfigs, "count/serviceaccounts": *serviceaccounts,
		"count/statefulsets.apps": *statefulsets, "count/templates.template.openshift.io": *templates, "openshift.io/imagestreams": *imagestreams}

	//LimitRange
	MinPodCpu               = resource.MustParse("25m")
	MinPodMem               = resource.MustParse("200M")
	min                     = v1.ResourceList{"cpu": MinPodCpu, "memory": MinPodMem}
	DefaultRequestPodCpu    = resource.MustParse("100m")
	DefaultRequestPodMem    = resource.MustParse("200M")
	defaultRequest          = v1.ResourceList{"cpu": DefaultRequestPodCpu, "memory": DefaultRequestPodMem}
	MaxLimitRequestRatioCpu = resource.NewQuantity(44, resource.DecimalSI)
	MaxLimitRequestRatioMem = resource.NewQuantity(29, resource.DecimalSI)
	maxLimitRequestRatio    = v1.ResourceList{"cpu": *MaxLimitRequestRatioCpu, "memory": *MaxLimitRequestRatioMem}
	Limits                  = v1.LimitRangeItem{
		Type:                 "Container",
		Min:                  min,
		Default:              defaultRequest, //default limit
		DefaultRequest:       defaultRequest,
		MaxLimitRequestRatio: maxLimitRequestRatio,
	}

	ZeroedQuota = v1.ResourceQuotaSpec{
		Hard: v1.ResourceList{"basic.storageclass.storage.k8s.io/requests.storage": *ZeroDecimal, "cpu": *ZeroDecimal, "memory": *ZeroDecimal, "pods": *ZeroDecimal, "requests.nvidia.com/gpu": *ZeroDecimal},
	}
)
