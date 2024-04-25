package resource

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	nvdevicelib "github.com/klueska/nvk8s-resourcemodel/pkg/nvdevice"
	currentresourceapi "github.com/klueska/nvk8s-resourcemodel/pkg/resource/current"
)

// PerGpuAllocatableDevices is an alias of nvdevicelib.PerGpuAllocatableDevices
type PerGpuAllocatableDevices nvdevicelib.PerGpuAllocatableDevices

// AllocatableDevices is an alias of nvdevicelib.AllocatableDevices
type AllocatableDevices nvdevicelib.AllocatableDevices

// GpuInfo is an alias of nvdevicelib.GpuInfo
type GpuInfo nvdevicelib.GpuInfo

// MigInfo is an alias of nvdevicelib.MigInfo
type MigInfo nvdevicelib.MigInfo

// ToNamedResourcesResourceModel converts a list of PerGpuAllocatableDevices to a NamedResources ResourceModel.
func (devices PerGpuAllocatableDevices) ToNamedResourcesResourceModel() ResourceModel {
	instances := devices.ToNamedResourceInstances()
	sharedLimits := devices.ToSharedLimits()
	model := ResourceModel{
		NamedResources: &NamedResourcesResources{
			Instances:    instances,
			SharedLimits: sharedLimits,
		},
	}
	return model
}

// ToNamedResourcesResourceModel converts a list of AllocatableDevices to a NamedResources ResourceModel.
func (devices AllocatableDevices) ToNamedResourcesResourceModel() ResourceModel {
	instances := devices.ToNamedResourceInstances()
	sharedLimits := devices.ToSharedLimits()
	model := ResourceModel{
		NamedResources: &NamedResourcesResources{
			Instances:    instances,
			SharedLimits: []NamedResourcesGroup{sharedLimits},
		},
	}
	return model
}

// ToNamedResourceInstances converts a list of PerGpuAllocatableDevices to a list of NamedResourcesInstances.
func (devices PerGpuAllocatableDevices) ToNamedResourceInstances() []NamedResourcesInstance {
	var instances []NamedResourcesInstance
	for _, perGpuDevices := range devices {
		instances = slices.Concat(instances, AllocatableDevices(perGpuDevices).ToNamedResourceInstances())
	}
	return instances
}

// ToNamedResourceInstances converts a list of AllocatableDevices to a list of NamedResourcesInstances.
func (devices AllocatableDevices) ToNamedResourceInstances() []NamedResourcesInstance {
	var instances []NamedResourcesInstance
	for _, device := range devices {
		var instance *NamedResourcesInstance
		if device.Mig != nil {
			instance = (*MigInfo)(device.Mig).ToNamedResourcesInstance()
		}
		if device.Gpu != nil {
			instance = (*GpuInfo)(device.Gpu).ToNamedResourcesInstance()
		}
		if instance != nil {
			instances = append(instances, *instance)
		}
	}
	return instances
}

// ToSharedLimits converts a list of PerGpuAllocatableDevices to a list of NamedResourcesGroups shared resource limits.
func (devices PerGpuAllocatableDevices) ToSharedLimits() []NamedResourcesGroup {
	var limits []NamedResourcesGroup
	for _, perGpuDevices := range devices {
		limits = append(limits, AllocatableDevices(perGpuDevices).ToSharedLimits())
	}
	return limits
}

// ToSharedLimits converts a list of AllocatableDevices to a NamedResourcesGroup of shared resource limits.
func (devices AllocatableDevices) ToSharedLimits() NamedResourcesGroup {
	var limits NamedResourcesGroup
	for _, device := range devices {
		var resources NamedResourcesGroup
		if device.Gpu != nil {
			resources = (*GpuInfo)(device.Gpu).getResources()
		}
		if device.Mig != nil {
			resources = (*MigInfo)(device.Mig).getResources()
		}
		for _, q := range resources.Quantities {
			limits.addOrReplaceQuantityIfLarger(&q)
		}
		for _, s := range resources.IntSets {
			limits.addOrReplaceIntSetIfLarger(&s)
		}
	}
	return limits
}

// addOrReplaceQuantityIfLarger is an internal function to conditionally update Quantities in a NamedResourcesGroup.
func (g *NamedResourcesGroup) addOrReplaceQuantityIfLarger(q *NamedResourcesQuantity) {
	for i := range g.Quantities {
		if q.Name == g.Quantities[i].Name {
			if q.Value.Cmp(*g.Quantities[i].Value) > 0 {
				*g.Quantities[i].Value = *q.Value
			}
			return
		}
	}
	g.Quantities = append(g.Quantities, *q)
}

// addOrReplaceIntSetIfLarger is an internal function to conditionally update IntSets in a NamedResourcesGroup.
func (g *NamedResourcesGroup) addOrReplaceIntSetIfLarger(s *NamedResourcesSet[int]) {
	for i := range g.IntSets {
		if s.Name == g.IntSets[i].Name {
			for _, item := range s.Items {
				if !slices.Contains(g.IntSets[i].Items, item) {
					g.IntSets[i].Items = append(g.IntSets[i].Items, item)
				}
			}
			return
		}
	}
	g.IntSets = append(g.IntSets, *s)
}

// ToNamedResourcesInstance converts a GpuInfo object to a NamedResourcesInstance.
func (gpu *GpuInfo) ToNamedResourcesInstance() *NamedResourcesInstance {
	currentInstance := (*currentresourceapi.GpuInfo)(gpu).ToNamedResourcesInstance()

	var attributes []NamedResourcesAttribute
	for _, attribute := range currentInstance.Attributes {
		switch attribute.Name {
		case "memory":
			break
		default:
			attributes = append(attributes, attribute)
		}
	}

	resources := []NamedResourcesGroup{
		(*GpuInfo)(gpu).getResources(),
	}

	newInstance := &NamedResourcesInstance{
		Name:       currentInstance.Name,
		Attributes: attributes,
		Resources:  resources,
	}

	return newInstance
}

// ToNamedResourcesInstance converts a MigInfo object to a NamedResourcesInstance.
func (mig *MigInfo) ToNamedResourcesInstance() *NamedResourcesInstance {
	parentInstance := (*currentresourceapi.GpuInfo)(mig.Parent).ToNamedResourcesInstance()

	attributes := []NamedResourcesAttribute{
		{
			Name: "mig-profile",
			NamedResourcesAttributeValue: NamedResourcesAttributeValue{
				StringValue: ptr.To(mig.Profile.String()),
			},
		},
	}
	for _, attribute := range parentInstance.Attributes {
		switch attribute.Name {
		case "product-name", "brand", "architecture", "cuda-compute-capability", "driver-version", "cuda-driver-version":
			attributes = append(attributes, attribute)
		}
	}

	resources := []NamedResourcesGroup{
		(*MigInfo)(mig).getResources(),
	}

	name := fmt.Sprintf("%s-mig-%s-%d", parentInstance.Name, mig.Profile, mig.MemorySlices.Start)

	instance := &NamedResourcesInstance{
		Name:       toRFC1123Compliant(name),
		Attributes: attributes,
		Resources:  resources,
	}

	return instance
}

// getResources gets the set of shared resources consumed by the GPU.
func (gpu *GpuInfo) getResources() NamedResourcesGroup {
	name := fmt.Sprintf("gpu-%v-shared-resources", gpu.Index)

	quantities := []NamedResourcesQuantity{
		{
			Name:  "memory",
			Value: resource.NewQuantity(int64(gpu.MemoryBytes), resource.BinarySI),
		},
	}

	group := NamedResourcesGroup{
		Name:       name,
		Quantities: quantities,
	}

	return group
}

// getResources gets the set of shared resources consumed by the MIG device.
func (mig *MigInfo) getResources() NamedResourcesGroup {
	name := fmt.Sprintf("gpu-%v-shared-resources", mig.Parent.Index)

	info := mig.GIProfileInfo
	quantities := []NamedResourcesQuantity{
		{
			Name:  "multiprocessors",
			Value: resource.NewQuantity(int64(info.MultiprocessorCount), resource.BinarySI),
		},
		{
			Name:  "copy-engines",
			Value: resource.NewQuantity(int64(info.CopyEngineCount), resource.BinarySI),
		},
		{
			Name:  "decoders",
			Value: resource.NewQuantity(int64(info.DecoderCount), resource.BinarySI),
		},
		{
			Name:  "encoders",
			Value: resource.NewQuantity(int64(info.EncoderCount), resource.BinarySI),
		},
		{
			Name:  "jpeg-engines",
			Value: resource.NewQuantity(int64(info.JpegCount), resource.BinarySI),
		},
		{
			Name:  "ofa-engines",
			Value: resource.NewQuantity(int64(info.OfaCount), resource.BinarySI),
		},
		{
			Name:  "memory",
			Value: resource.NewQuantity(int64(info.MemorySizeMB*1024*1024), resource.BinarySI),
		},
	}

	var memorySlices []int
	for i := 0; i < int(mig.MemorySlices.Size); i++ {
		memorySlices = append(memorySlices, int(mig.MemorySlices.Start)+i)
	}
	memorySlicesSet := NamedResourcesSet[int]{
		Name:  "memory-slices",
		Items: memorySlices,
	}

	group := NamedResourcesGroup{
		Name:       name,
		Quantities: quantities,
		IntSets:    []NamedResourcesSet[int]{memorySlicesSet},
	}

	return group
}

// toRFC1123Compliant converts the incoming string to a valid RFC1123 DNS domain name.
func toRFC1123Compliant(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with '-'
	re := regexp.MustCompile(`[^a-z0-9-.]`)
	name = re.ReplaceAllString(name, "-")

	// Trim leading/trailing '-'
	name = strings.Trim(name, "-")

	// Trim trailing '.'
	name = strings.TrimSuffix(name, ".")

	// Truncate to 253 characters
	if len(name) > 253 {
		name = name[:253]
	}

	return name
}
