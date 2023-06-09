package v1

import (
	"os"
	"strconv"
)

type Phase string

const MetaGroup = "dana.hns.io/"

var MaxSNS, _ = strconv.Atoi(os.Getenv("MAX_SNS_IN_HIERARCHY"))

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

const (
	NsFinalizer = MetaGroup + "delete-sns"
	RbFinalizer = MetaGroup + "delete-rb"
)

const (
	SelfOffset   = 0
	ParentOffset = -1
	ChildOffset  = 1
)

const (
	Hns          = MetaGroup + "subnamespace"
	Parent       = MetaGroup + "parent"
	ResourcePool = MetaGroup + "resourcepool"
)

const (
	Role                 = MetaGroup + "role"
	Depth                = MetaGroup + "depth"
	CrqSelector          = MetaGroup + "crq-selector"
	RootCrqSelector      = CrqSelector + "-0"
	SnsPointer           = MetaGroup + "sns-pointer"
	RqDepth              = MetaGroup + "rq-depth"
	IsRq                 = MetaGroup + "is-rq"
	IsSecondaryRoot      = MetaGroup + "is-secondary-root"
	IsUpperRp            = MetaGroup + "is-upper-rp"
	UpperRp              = MetaGroup + "upper-rp"
	CrqPointer           = MetaGroup + "crq-pointer"
	DisplayName          = MetaGroup + "display-name"
	OpenShiftDisplayName = "openshift.io/display-name"
)

const (
	MaxRetries   = 600
	SleepTimeout = 500
)
