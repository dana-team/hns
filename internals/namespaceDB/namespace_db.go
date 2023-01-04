package namespaceDB

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	v1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"sync"

	danav1 "github.com/dana-team/hns/api/v1"
	corev1 "k8s.io/api/core/v1"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type NamespaceDB struct {
	nsparentCrq map[string][]string
	mutex       *sync.RWMutex
}

// InitDB builds in memory db contains
// key namespaces with values of all namespaces' names under the key hierarchy.
// key namespace is the first namespace in the hierarchy that has CRQ and not RQ
func InitDB(client goclient.Client, log logr.Logger) (*NamespaceDB, error) {
	rootns := &corev1.Namespace{}
	ndb := &NamespaceDB{nsparentCrq: make(map[string][]string), mutex: &sync.RWMutex{}}
	nslist := corev1.NamespaceList{}
	snslist := v1.SubnamespaceList{}
	nsWithSns := corev1.NamespaceList{}

	if err := client.List(context.Background(), &nslist); err != nil {
		return ndb, err
	}
	if err := client.List(context.Background(), &snslist); err != nil {
		return ndb, err
	}
	for _, ns := range nslist.Items {
		if isSubnamespace(snslist, ns.Name) {
			nsWithSns.Items = append(nsWithSns.Items, ns)
		}
	}
	if len(nsWithSns.Items) > 0 {
		rootns = LocateNS(nslist, nsWithSns.Items[0].Annotations[v1.RootCrqSelector])
	}
	for _, ns := range nsWithSns.Items {
		if ok := ns.Annotations[v1.Role] == v1.Leaf; ok {
			err := addHierarchy(ns, nslist, ndb, rootns)
			if err != nil {
				return ndb, err
			}
		}
	}
	return ndb, nil
}

func LocateNS(nsList corev1.NamespaceList, nsName string) *corev1.Namespace {
	for _, ns := range nsList.Items {
		if ns.Name == nsName {
			return &ns
		}
	}
	return nil
}

func isSubnamespace(snslist v1.SubnamespaceList, nsname string) bool {
	for _, sns := range snslist.Items {
		if sns.Name == nsname {
			return true
		}
	}
	return false
}

func addHierarchy(ns corev1.Namespace, nsList corev1.NamespaceList, ndb *NamespaceDB, rootns *corev1.Namespace) error {
	nslistup, _ := utils.GetNsListUpEfficient(ns, rootns.Name, nsList)
	sort.Slice(nslistup, func(i, j int) bool {
		return nslistup[i].Annotations[v1.Depth] < nslistup[j].Annotations[v1.Depth]
	})
	rqdepth, _ := strconv.Atoi(rootns.Annotations[v1.RqDepth])
	if len(nslistup) > rqdepth {
		keyname := nslistup[rqdepth].GetName()
		if !ndb.isKeyExist(keyname) {
			ndb.addNsToKey(keyname, keyname)
		}
		for _, namespace := range nslistup[rqdepth+1:] {
			if !ndb.valInKeyExist(keyname, namespace.GetName()) {
				ndb.addNsToKey(keyname, namespace.GetName())
			}
		}
	}
	return nil
}

//isKeyExist checks whether a key namespace exists in the db
func (ndb *NamespaceDB) isKeyExist(key string) bool {
	ndb.mutex.RLock()
	defer ndb.mutex.RUnlock()
	if _, ok := ndb.nsparentCrq[key]; ok {
		return true
	}
	return false
}

//valInKeyExist checks whether a value in specific key namespace
func (ndb *NamespaceDB) valInKeyExist(key string, value string) bool {
	ndb.mutex.RLock()
	defer ndb.mutex.RUnlock()
	for _, val := range ndb.nsparentCrq[key] {
		if val == value {
			return true
		}
	}
	return false
}

//addNsToKey adds namespace to its key namespace in the db
func (ndb *NamespaceDB) addNsToKey(key string, ns string) {
	ndb.mutex.Lock()
	defer ndb.mutex.Unlock()
	if key == ns {
		ndb.nsparentCrq[key] = []string{}
	} else {
		if nslist, ok := ndb.nsparentCrq[key]; ok {
			nslist = append(nslist, ns)
			ndb.nsparentCrq[key] = nslist
		} else {
			nslist := []string{ns}
			ndb.nsparentCrq[key] = nslist
		}
	}
}

//addNs finds and adds namespace to its key namespace if exist
//if doesnt it checks whether the namespace is the key namespace
//otherwise it does nothing since it doesnt need to be in the db
func AddNs(ndb *NamespaceDB, client goclient.Client, sns *danav1.Subnamespace) error {
	keyns := ndb.GetKey(sns.Namespace)
	if keyns != "" {
		ndb.addNsToKey(keyns, sns.Name)
		return nil
	}
	crq := quotav1.ClusterResourceQuota{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: sns.Name}, &crq); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}
	ndb.addNsToKey(sns.Name, sns.Name)
	return nil
}

//MigrateNsHierarchy migrates namespace and its children hierarchy from key namespace to another key
func MigrateNsHierarchy(ndb *NamespaceDB, client goclient.Client, snsname string, targetnsname string) error {
	oldkeyns := ndb.GetKey(snsname)
	newkeyns := ndb.GetKey(targetnsname)
	if oldkeyns == "" || newkeyns == "" {
		return fmt.Errorf("sub namespace has no key")
	}
	crq := quotav1.ClusterResourceQuota{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: snsname}, &crq); err != nil {
		return err
	}
	namespaces := crq.Status.Namespaces
	for _, namespace := range namespaces {
		nsname := namespace.Namespace
		ndb.removeNs(nsname, oldkeyns)
		ndb.addNsToKey(newkeyns, nsname)
	}
	return nil
}

//removeNs removes a namespace from a key
func (ndb *NamespaceDB) removeNs(nsname string, key string) {
	ndb.mutex.Lock()
	defer ndb.mutex.Unlock()
	for i, namespace := range ndb.nsparentCrq[key] {
		if namespace == nsname {
			ndb.nsparentCrq[key] = append(ndb.nsparentCrq[key][:i], ndb.nsparentCrq[key][i+1:]...)
			return
		}
	}
}

//getKey takes namespace name
//and returns sns' key namespace from its hierarchy if exists otherwise it returns empty string
func (ndb *NamespaceDB) GetKey(ns string) string {
	ndb.mutex.RLock()
	defer ndb.mutex.RUnlock()
	for key, namespaces := range ndb.nsparentCrq {
		if key == ns {
			return key
		}
		for _, namespace := range namespaces {
			if namespace == ns {
				return key
			}
		}
	}
	return ""
}

//GetKeyCount returns the number of namespaces under the hierarchy of given key namespace
func (ndb *NamespaceDB) GetKeyCount(key string) int {
	ndb.mutex.RLock()
	defer ndb.mutex.RUnlock()
	if ns, ok := ndb.nsparentCrq[key]; ok {
		return len(ns)
	}
	return 0
}
