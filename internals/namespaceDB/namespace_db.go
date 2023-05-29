package namespaceDB

import (
	"context"
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"

	"sort"
	"strconv"
	"sync"
)

// NamespaceDB is an in-memory DB that contains a map with a key that is a string representing the first
// namespace in a hierarchy that is bound to a CRQ and not RQ, and a value that is a slice of all namespaces
// which are under this particular key in the hierarchy
type NamespaceDB struct {
	crqForest map[string][]string
	mutex     *sync.RWMutex
}

// createClient returns a new client
func createClient(scheme *runtime.Scheme) (client.Client, error) {
	cfg := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})

	return k8sClient, err
}

// InitDB initializes a new NamespaceDB instance
func InitDB(scheme *runtime.Scheme, logger logr.Logger) (*NamespaceDB, error) {
	logger.Info("initializing namespaceDB")

	nDB := &NamespaceDB{crqForest: make(map[string][]string), mutex: &sync.RWMutex{}}
	rootNS := &corev1.Namespace{}
	nsList := corev1.NamespaceList{}
	snsList := danav1.SubnamespaceList{}
	nsWithSns := corev1.NamespaceList{}

	client, err := createClient(scheme)
	if err != nil {
		return nDB, err
	}

	if err := client.List(context.Background(), &nsList); err != nil {
		return nDB, err
	}

	if err := client.List(context.Background(), &snsList); err != nil {
		return nDB, err
	}

	// filter the namespace list to only include namespaces that have subnamespaces
	for _, ns := range nsList.Items {
		if isSNS(snsList, ns.Name) {
			nsWithSns.Items = append(nsWithSns.Items, ns)
		}
	}

	if len(nsWithSns.Items) > 0 {
		rootNS = LocateNS(nsList, nsWithSns.Items[0].Annotations[danav1.RootCrqSelector])
	}

	for _, ns := range nsWithSns.Items {
		if ok := ns.Annotations[danav1.Role] == danav1.Leaf; ok {
			if err := addHierarchy(ns, nsList, nDB, rootNS); err != nil {
				return nDB, fmt.Errorf("failed to add hierarchy for '%s': "+err.Error(), ns.Name)
			}
			logger.Info("successfully added hierarchy", "namespace", ns.Name)
		}
	}

	return nDB, nil
}

// LocateNS locates a namespace by name in a given namespace list
func LocateNS(nsList corev1.NamespaceList, nsName string) *corev1.Namespace {
	for _, ns := range nsList.Items {
		if ns.Name == nsName {
			return &ns
		}
	}
	return nil
}

// isSNS checks if the provided namespace exists in the given subnamespace list
func isSNS(snsList danav1.SubnamespaceList, nsName string) bool {
	for _, sns := range snsList.Items {
		if sns.Name == nsName {
			return true
		}
	}
	return false
}

// addHierarchy adds given a namespace and all its ancestor namespaces to the appropriate key in the DB
func addHierarchy(ns corev1.Namespace, nsList corev1.NamespaceList, ndb *NamespaceDB, rootNS *corev1.Namespace) error {
	nsListUp, _ := GetNsListUp(ns, rootNS.Name, nsList)

	sort.Slice(nsListUp, func(i, j int) bool {
		return nsListUp[i].Annotations[danav1.Depth] < nsListUp[j].Annotations[danav1.Depth]
	})

	rqDepth, _ := strconv.Atoi(rootNS.Annotations[danav1.RqDepth])

	// if the namespace is at a depth greater than the rqDepth, then it means
	// that the namespace has a CRQ bound to it, and so add it to the appropriate key in the NamespaceDB
	if len(nsListUp) > rqDepth {
		keyName := nsListUp[rqDepth].GetName()

		if !ndb.doesKeyExist(keyName) {
			if err := ndb.addNSToKey(keyName, keyName); err != nil {
				return fmt.Errorf("failed to add namespace '%s' to key '%s': "+err.Error(), keyName, keyName)
			}
		}

		for _, namespace := range nsListUp[rqDepth+1:] {
			if !ndb.valInKeyExist(keyName, namespace.GetName()) {
				if err := ndb.addNSToKey(keyName, namespace.GetName()); err != nil {
					return fmt.Errorf("failed to add namespace '%s' to key '%s': "+err.Error(), namespace.GetName(), keyName)
				}
			}
		}
	}

	return nil
}

// GetNsListUp creates a slice of all namespaces in the hierarchy from `ns` to `rootNS`
func GetNsListUp(ns corev1.Namespace, rootNS string, nsList corev1.NamespaceList) ([]corev1.Namespace, error) {
	displayName := ns.GetAnnotations()[danav1.DisplayName]
	nsArray := strings.Split(displayName, "/")

	index, err := utils.IndexOf(rootNS, nsArray)
	if err != nil {
		return nil, err
	}
	snsArr := nsArray[index:]

	var nsListUp []corev1.Namespace
	for i := len(snsArr) - 1; i >= 1; i-- {
		ns := LocateNS(nsList, snsArr[i])
		nsListUp = append(nsListUp, *ns)
	}

	return nsListUp, nil
}

// doesKeyExist checks whether a key with a specific name exists in the db
func (ndb *NamespaceDB) doesKeyExist(key string) bool {
	ndb.mutex.RLock()
	defer ndb.mutex.RUnlock()

	if _, ok := ndb.crqForest[key]; ok {
		return true
	}
	return false
}

// valInKeyExist checks if a value exists in a key's slice of values
func (ndb *NamespaceDB) valInKeyExist(key string, value string) bool {
	ndb.mutex.RLock()
	defer ndb.mutex.RUnlock()

	for _, val := range ndb.crqForest[key] {
		if val == value {
			return true
		}
	}
	return false
}

// addNSToKey adds namespace to its key namespace in the db
func (ndb *NamespaceDB) addNSToKey(key string, ns string) error {
	ndb.mutex.Lock()
	defer ndb.mutex.Unlock()

	if key == ns {
		ndb.crqForest[key] = []string{}
	} else {
		if nsList, ok := ndb.crqForest[key]; ok {
			nsList = append(nsList, ns)
			ndb.crqForest[key] = nsList
		} else {
			nsList := []string{ns}
			ndb.crqForest[key] = nsList
		}
	}

	if _, ok := ndb.crqForest[key]; !ok {
		return fmt.Errorf("key '%s' does not exist in NamespaceDB", key)
	}

	return nil
}

// AddNS adds a namespace to its key if such exist, otherwise it checks whether the namespace
// should be the key itself and adds it to the DB
func AddNS(ctx context.Context, nDB *NamespaceDB, client client.Client, sns *danav1.Subnamespace) error {
	logger := log.FromContext(ctx)
	keyNS := nDB.GetKey(sns.Namespace)

	if keyNS != "" {
		if err := nDB.addNSToKey(keyNS, sns.Name); err != nil {
			return fmt.Errorf("failed to add namespace '%s' to key '%s': "+err.Error(), sns.Name, keyNS)
		}
		logger.Info("added namespace under key in namespaceDB", "namespace", sns.Name, "key", keyNS)
		return nil
	}

	crq := quotav1.ClusterResourceQuota{}
	if err := client.Get(ctx, types.NamespacedName{Name: sns.Name}, &crq); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	// if a key does not already exist for the namespace, but the namespace has a CRQ then it means
	// that the namespace itself should be the key in the DB
	if err := nDB.addNSToKey(sns.Name, sns.Name); err != nil {
		return fmt.Errorf("failed to add namespace '%s' to key '%s': "+err.Error(), sns.Name, sns.Name)
	}
	logger.Info("added namespace under key in namespaceDB", "namespace", sns.Name, "key", sns.Name)

	return nil
}

// MigrateNSHierarchy migrates namespace and its children hierarchy from one key to another key
func MigrateNSHierarchy(ctx context.Context, ndb *NamespaceDB, client client.Client, snsName string, destNSName string) error {
	oldKeyNS := ndb.GetKey(snsName)
	newKeyNS := ndb.GetKey(destNSName)

	if oldKeyNS == "" {
		return fmt.Errorf("failed to find key '%s' in DB for namespace '%s'", oldKeyNS, snsName)
	}

	if newKeyNS == "" {
		return fmt.Errorf("failed to find key '%s' in DB for namespace '%s'", newKeyNS, destNSName)
	}

	ns, err := utils.NewObjectContext(ctx, client, types.NamespacedName{Name: snsName}, &corev1.Namespace{})
	if err != nil {
		return err
	}

	// get all the children of the source namespace, including the ns itself, and iterate over them
	childrenList := utils.GetAllChildren(ns)

	for _, child := range childrenList {
		childName := child.GetName()

		if err := ndb.RemoveNS(childName, oldKeyNS); err != nil {
			return fmt.Errorf("removing namespace '%s' from key '%s' failed: "+err.Error(), childName, oldKeyNS)
		}

		if err := ndb.addNSToKey(newKeyNS, childName); err != nil {
			return fmt.Errorf("adding namespace '%s' to key '%s' failed: "+err.Error(), childName, newKeyNS)
		}
	}
	return nil
}

// RemoveNS removes a namespace from the slice of namespaces that belongs to a key
func (ndb *NamespaceDB) RemoveNS(nsname string, key string) error {
	ndb.mutex.Lock()
	defer ndb.mutex.Unlock()

	if _, ok := ndb.crqForest[key]; !ok {
		return fmt.Errorf("key '%s' does not exist in NamespaceDB", key)
	}

	for i, namespace := range ndb.crqForest[key] {
		if namespace == nsname {
			ndb.crqForest[key] = append(ndb.crqForest[key][:i], ndb.crqForest[key][i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("namespace '%s' not found in key '%s'", nsname, key)
}

// GetKey retrieves the key that the provided namespace belongs to
func (ndb *NamespaceDB) GetKey(ns string) string {
	ndb.mutex.RLock()
	defer ndb.mutex.RUnlock()

	for key, namespaces := range ndb.crqForest {
		if key == ns {
			return key
		}

		for _, namespace := range namespaces {
			if namespace == ns {
				return key
			}
		}
	}

	// if the namespace does not belong to any key, return an empty string
	return ""
}

// DeleteKey deletes a key from the database
func (ndb *NamespaceDB) DeleteKey(key string) {
	ndb.mutex.RLock()
	defer ndb.mutex.RUnlock()

	delete(ndb.crqForest, key)
}

// GetKeyCount returns the number of namespaces that belong to a specific key
func (ndb *NamespaceDB) GetKeyCount(key string) int {
	ndb.mutex.RLock()
	defer ndb.mutex.RUnlock()

	if ns, ok := ndb.crqForest[key]; ok {
		return len(ns)
	}

	return 0
}
