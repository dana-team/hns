package rbutils

import (
	"reflect"
	"strings"

	"github.com/dana-team/hns/internal/common"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultSA  = "default"
	builderSA  = "builder"
	deployerSA = "deployer"
)

// IsRoleBinding returns true if an object is of type RoleBinding.
func IsRoleBinding(object client.Object) bool {
	return reflect.TypeOf(object) == reflect.TypeOf(&rbacv1.RoleBinding{})
}

// IsHNSRelated checks if a RoleBinding object is valid.
func IsHNSRelated(roleBinding client.Object) bool {
	if !IsRoleBinding(roleBinding) {
		return false
	}

	if len(roleBinding.(*rbacv1.RoleBinding).Subjects) == 0 {
		return false
	}

	rbKind := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Kind
	if rbKind == "Group" {
		rbName := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Name
		if strings.HasPrefix(rbName, "system") {
			return false
		}
	}

	if rbKind == "ServiceAccount" {
		if roleBinding.(*rbacv1.RoleBinding).Subjects[0].Namespace == "" {
			return false
		}
	}

	rbName := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Name
	serviceAccountNames := []string{defaultSA, builderSA, deployerSA}

	return !common.ContainsString(serviceAccountNames, rbName)
}

// Subjects returns a slice of subjects from a roleBinding.
func Subjects(roleBinding client.Object) []rbacv1.Subject {
	if !IsRoleBinding(roleBinding) {
		return nil
	}

	if roleBinding.(*rbacv1.RoleBinding).Subjects[0].Namespace == "" {
		roleBinding.(*rbacv1.RoleBinding).Subjects[0].Namespace = roleBinding.(*rbacv1.RoleBinding).Namespace
	}

	return roleBinding.(*rbacv1.RoleBinding).Subjects
}

// RoleRef returns the roleRef of a roleBinding.
func RoleRef(roleBinding client.Object) rbacv1.RoleRef {
	if !IsRoleBinding(roleBinding) {
		return rbacv1.RoleRef{}
	}

	return roleBinding.(*rbacv1.RoleBinding).RoleRef
}

// IsServiceAccount returns true if a subject in a RoleBinding is a ServiceAccount and false otherwise.
func IsServiceAccount(roleBinding client.Object) bool {
	if !IsHNSRelated(roleBinding) {
		return false
	}

	rbKind := roleBinding.(*rbacv1.RoleBinding).Subjects[0].Kind
	return rbKind == "ServiceAccount"
}

// UpdateNamespaceHNSViewCRBSubjects updates the subjects of a ClusterRoleBinding.
func UpdateNamespaceHNSViewCRBSubjects(hnsViewClusterRoleBinding *objectcontext.ObjectContext, subjects []rbacv1.Subject) error {
	return hnsViewClusterRoleBinding.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated ClusterRoleBinding subjects", hnsViewClusterRoleBinding.Name())
		hnsViewClusterRoleBinding.Object.(*rbacv1.ClusterRoleBinding).Subjects = subjects
		return object, log
	})
}

// NamespaceHNSViewCRB returns the HNS View ClusterRoleBinding associated with a namespace.
func NamespaceHNSViewCRB(rbObject *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	namespace := rbObject.Namespace()
	nsClusterRoleBindingName := HNSViewName(namespace)

	nsClusterRoleBinding, err := objectcontext.New(rbObject.Ctx, rbObject.Client, types.NamespacedName{Name: nsClusterRoleBindingName}, &rbacv1.ClusterRoleBinding{})
	if err != nil {
		return nil, err
	}

	return nsClusterRoleBinding, nil
}

// IsSubjectInSubjects returns whether a subject is in a slice of subjects.
func IsSubjectInSubjects(subjects []rbacv1.Subject, subjectToFind rbacv1.Subject) bool {
	for _, subject := range subjects {
		if subjectsEqual(subject, subjectToFind) {
			return true
		}
	}

	return false
}

// subjectsEqual returns true if two subjects are equal and false otherwise.
func subjectsEqual(subject1 rbacv1.Subject, subject2 rbacv1.Subject) bool {
	return subject1.Name == subject2.Name && subject1.Kind == subject2.Kind && subject1.APIGroup == subject2.APIGroup
}

// Compose returns a RoleBinding object based on the given parameters.
func Compose(rbName string, namespace string, subjects []rbacv1.Subject, ref rbacv1.RoleRef) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: namespace,
		},
		Subjects: subjects,
		RoleRef:  ref,
	}
}

// HNSViewName returns the name of the HNS view role associated with a namespace.
func HNSViewName(nsName string) string {
	return nsName + "-hns-view"
}

// CreateHNSView creates the cluster role and cluster role binding HNS objects
// associated with the namespace.
func CreateHNSView(nsObject *objectcontext.ObjectContext) error {
	nsHnsViewClusterRoleObj, nsHnsViewClusterRoleBindingObj, err := NamespaceHNSView(nsObject)
	if err != nil {
		return err
	}

	if err := nsHnsViewClusterRoleObj.EnsureCreate(); err != nil {
		return err
	}

	return nsHnsViewClusterRoleBindingObj.EnsureCreate()
}

// Create takes care of creating a RoleBinding object in a namespace from an existing roleBinding.
func Create(nsObject *objectcontext.ObjectContext, roleBinding rbacv1.RoleBinding) error {
	rb := Compose(roleBinding.Name, nsObject.Name(), roleBinding.Subjects, roleBinding.RoleRef)

	rbObject, err := objectcontext.New(nsObject.Ctx, nsObject.Client, types.NamespacedName{}, rb)
	if err != nil {
		return err
	}

	err = rbObject.CreateObject()

	return err
}

// NamespaceHNSView returns the cluster role and cluster role binding HNS objects
// associated with the namespace.
func NamespaceHNSView(nsObject *objectcontext.ObjectContext) (*objectcontext.ObjectContext, *objectcontext.ObjectContext, error) {
	nsClusterRoleName := HNSViewName(nsObject.Name())
	nsClusterRole := composeNsHnsViewClusterRole(nsObject.Name())
	nsClusterRoleBinding := composeNsHnsViewClusterRoleBinding(nsObject.Name())

	nsHnsViewClusterRoleObj, err := objectcontext.New(nsObject.Ctx, nsObject.Client, types.NamespacedName{Name: nsClusterRoleName}, nsClusterRole)
	if err != nil {
		return nil, nil, err
	}

	nsHnsViewClusterRoleBindingObj, err := objectcontext.New(nsObject.Ctx, nsObject.Client, types.NamespacedName{Name: nsClusterRoleName}, nsClusterRoleBinding)
	if err != nil {
		return nil, nil, err
	}

	return nsHnsViewClusterRoleObj, nsHnsViewClusterRoleBindingObj, nil
}

// composeNsHnsViewClusterRole returns a ClusterRole object based on the given parameters.
func composeNsHnsViewClusterRole(namespaceName string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: HNSViewName(namespaceName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"dana.hns.io"},
				Resources: []string{"subnamespaces"},

				ResourceNames: []string{namespaceName}}, {
				Verbs:     []string{"get"},
				APIGroups: []string{"quota.openshift.io"},
				Resources: []string{"clusterresourcequotas"},

				ResourceNames: []string{namespaceName}}, {
				Verbs:     []string{"list"},
				APIGroups: []string{"quota.openshift.io"},
				Resources: []string{"clusterresourcequotas"},

				ResourceNames: []string{namespaceName}}, {
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"resourcequotas"},

				ResourceNames: []string{namespaceName}}, {
				Verbs:     []string{"list"},
				APIGroups: []string{""},
				Resources: []string{"resourcequotas"},
			},
		},
	}
}

// composeNsHnsViewClusterRoleBinding returns a ClusterRoleBinding object based on the given parameters.
func composeNsHnsViewClusterRoleBinding(namespaceName string) *rbacv1.ClusterRoleBinding {
	name := HNSViewName(namespaceName)
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name,
		},
	}
}
