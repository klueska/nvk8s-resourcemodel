package resource

import (
	resourceapi "k8s.io/api/resource/v1alpha2"
	"k8s.io/apimachinery/pkg/api/resource"
)

// NamedResourcesAttribute is an alias of resourceapi.NamedResourcesAttribute
type NamedResourcesAttribute = resourceapi.NamedResourcesAttribute

// NamedResourcesAttributeValue is an alias of resourceapi.NamedResourcesAttributeValue
type NamedResourcesAttributeValue = resourceapi.NamedResourcesAttributeValue

// NamedResourcesQuantity represents a named quantity of resources.
// +k8s:deepcopy-gen=true
type NamedResourcesQuantity struct {
	// Name is the name of the resource represented by this quantity.
	// It must be a DNS subdomain.
	Name string `json:"name" protobuf:"bytes,1,name=name"`

	// Value is the actual quantity of resources.
	Value *resource.Quantity `json:"value" protobuf:"bytes,2,name=value"`
}

// NamedResourcesIntSet represents a named list of discrete integers.
// +k8s:deepcopy-gen=true
type NamedResourcesIntSet struct {
	// Name is the name of the resource represented by this quantity.
	// It must be a DNS subdomain.
	Name string `json:"name" protobuf:"bytes,1,name=name"`

	// Items is the actual set of ints.
	//
	// +listType=set
	Items []int `json:"items" protobuf:"bytes,2,name=items"`
}

// NamedResourcesStringSet represents a named list of discrete strings.
// +k8s:deepcopy-gen=true
type NamedResourcesStringSet struct {
	// Name is the name of the resource represented by this quantity.
	// It must be a DNS subdomain.
	Name string `json:"name" protobuf:"bytes,1,name=name"`

	// Items is the actual set of strings.
	//
	// +listType=set
	Items []string `json:"items" protobuf:"bytes,2,name=items"`
}

// NamedResourcesNamedResourceGroup represents a named group of resources (quantites and sets).
// +k8s:deepcopy-gen=true
type NamedResourcesGroup struct {
	// Name is unique identifier among all resource groups managed by
	// the driver on the node. It must be a DNS subdomain.
	Name string `json:"name" protobuf:"bytes,1,name=name"`

	// Quantities represents the list of all named resource quantities in the
	// resource group.
	//
	// +listType=atomic
	// +optional
	Quantities []NamedResourcesQuantity `json:"quantities,omitempty" protobuf:"bytes,2,opt,name=quantities"`

	// StringSets represents the list of all named resource sets that contains
	// strings in the resource group.
	//
	// +listType=atomic
	// +optional
	StringSets []NamedResourcesStringSet `json:"stringSets,omitempty" protobuf:"bytes,3,opt,name=stringSets"`

	// IntSets represents the list of all named resource sets that contains
	// ints in the resource group.
	//
	// +listType=atomic
	// +optional
	IntSets []NamedResourcesIntSet `json:"intSets,omitempty" protobuf:"bytes,4,opt,name=intSets"`
}

// ResourceModel must have one and only one field set.
// +k8s:deepcopy-gen=true
type ResourceModel struct {
	// NamedResources describes available resources using the named resources model.
	//
	// +optional
	NamedResources *NamedResourcesResources `json:"namedResources,omitempty" protobuf:"bytes,1,opt,name=namedResources"`
}

// NamedResourcesResources is used in ResourceModel.
// +k8s:deepcopy-gen=true
type NamedResourcesResources struct {
	// The list of all individual resources instances currently available.
	//
	// +listType=atomic
	Instances []NamedResourcesInstance `json:"instances" protobuf:"bytes,1,name=instances"`

	// The list of all shared resources limits that are referenced by one or
	// more instances.
	//
	// +listType=atomic
	// +optional
	SharedLimits []NamedResourcesGroup `json:"sharedLimits,omitempty" protobuf:"bytes,2,opt,name=sharedLimits"`
}

// NamedResourcesInstance represents one individual hardware instance that can be selected based
// on its attributes.
// +k8s:deepcopy-gen=true
type NamedResourcesInstance struct {
	// Name is unique identifier among all resource instances managed by
	// the driver on the node. It must be a DNS subdomain.
	Name string `json:"name" protobuf:"bytes,1,name=name"`

	// Attributes defines the attributes of this resource instance.
	// The name of each attribute must be unique.
	//
	// +listType=atomic
	// +optional
	Attributes []NamedResourcesAttribute `json:"attributes,omitempty" protobuf:"bytes,2,opt,name=attributes"`

	// Resources defines the set of resources this instance consumes when
	// allocated.
	//
	// +listType=atomic
	// +optional
	Resources []NamedResourcesGroup `json:"resources,omitempty" protobuf:"bytes,3,opt,name=resources"`
}
