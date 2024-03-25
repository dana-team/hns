package quota

import (
	"slices"

	"github.com/dana-team/hns/internal/objectcontext"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ResourceListEqual gets two ResourceLists and returns whether their specs are equal.
func ResourceListEqual(resourceListA, resourceListB corev1.ResourceList) bool {
	if len(resourceListA) != len(resourceListB) {
		return false
	}

	for key, value1 := range resourceListA {
		value2, found := resourceListB[key]
		if !found {
			return false
		}
		if value1.Cmp(value2) != 0 {
			return false
		}
	}

	return true
}

// ResourceQuotaSpecEqual gets two ResourceQuotaSpecs and returns whether their specs are equal.
func ResourceQuotaSpecEqual(resourceQuotaSpecA, resourceQuotaSpecB corev1.ResourceQuotaSpec) bool {
	var resources []string

	for resourceName := range ZeroedQuota.Hard {
		resources = append(resources, resourceName.String())
	}

	resourceQuotaSpecAFiltered := filterResources(resourceQuotaSpecA.Hard, resources)
	resourceQuotaSpecBFiltered := filterResources(resourceQuotaSpecB.Hard, resources)

	return ResourceListEqual(resourceQuotaSpecAFiltered, resourceQuotaSpecBFiltered)
}

// filterResources filters the given resourceList and returns a new resourceList
// that contains only the resources specified in the controlled resources list.
func filterResources(resourcesList corev1.ResourceList, resources []string) corev1.ResourceList {
	filteredList := corev1.ResourceList{}

	for resourceName, quantity := range resourcesList {
		if slices.Contains(resources, resourceName.String()) {
			addResourcesToList(&filteredList, quantity, resourceName.String())
		}
	}
	return filteredList
}

// addResourcesToList adds the given quantity of a resource with the specified name to the resource list.
// If the resource with the same name already exists in the list, it adds the quantity to the existing resource.
func addResourcesToList(resourcesList *corev1.ResourceList, quantity resource.Quantity, name string) {
	for resourceName, resourceQuantity := range *resourcesList {
		if name == string(resourceName) {
			resourceQuantity.Add(quantity)
			(*resourcesList)[resourceName] = resourceQuantity
			return
		}
	}
	(*resourcesList)[corev1.ResourceName(name)] = quantity
}

// GetQuotaObjectsListResources returns a ResourceList with all the resources of the given objects summed up.
func GetQuotaObjectsListResources(quotaObjs []*objectcontext.ObjectContext) corev1.ResourceList {
	resourcesList := corev1.ResourceList{}

	for _, quotaObj := range quotaObjs {
		for quotaObjResource, quotaObjQuantity := range GetQuotaObjectSpec(quotaObj.Object).Hard {
			addQuantityToResourceList(resourcesList, quotaObjResource, quotaObjQuantity)
		}
	}
	return resourcesList
}

// addQuantityToResourceList adds together quantities of a ResourceList.
func addQuantityToResourceList(resourceList corev1.ResourceList, resourceName corev1.ResourceName, quantity resource.Quantity) {
	if currentQuantity, ok := resourceList[resourceName]; ok {
		currentQuantity.Add(quantity)
		resourceList[resourceName] = currentQuantity
	} else {
		resourceList[resourceName] = quantity
	}
}
