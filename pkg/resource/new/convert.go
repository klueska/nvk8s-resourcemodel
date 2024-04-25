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

// consumableQuantities is an internal type for storing the quantifiable resources of a device.
type consumableQuantities struct {
	MultiprocessorCount int64
	CopyEngineCount     int64
	DecoderCount        int64
	EncoderCount        int64
	JpegCount           int64
	OfaCount            int64
	MemoryBytes         int64
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
		limits = slices.Concat(limits, AllocatableDevices(perGpuDevices).ToSharedLimits())
	}
	return limits
}

// ToSharedLimits converts a list of AllocatableDevices to a list of NamedResourcesGroups shared resource limits.
func (devices AllocatableDevices) ToSharedLimits() []NamedResourcesGroup {
	var allLimits []NamedResourcesGroup
	for _, device := range devices {
		var limits []NamedResourcesGroup
		if device.Mig != nil {
			continue
		}
		if device.Gpu != nil {
			limits = (*GpuInfo)(device.Gpu).getResources()
		}
		if limits != nil {
			allLimits = slices.Concat(allLimits, limits)
		}
	}
	return allLimits
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

	newInstance := &NamedResourcesInstance{
		Name:       currentInstance.Name,
		Attributes: attributes,
		Resources:  (*GpuInfo)(gpu).getResources(),
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

	name := fmt.Sprintf("%s-mig-%s-%d", parentInstance.Name, mig.Profile, mig.MemorySlices.Start)
	instance := &NamedResourcesInstance{
		Name:       toRFC1123Compliant(name),
		Attributes: attributes,
		Resources:  (*MigInfo)(mig).getResources(),
	}

	return instance
}

// getResources gets the set of shared resources consumed by the GPU.
func (gpu *GpuInfo) getResources() []NamedResourcesGroup {
	quantities := consumableQuantities{
		MultiprocessorCount: int64(gpu.Attributes.MultiprocessorCount),
		CopyEngineCount:     int64(gpu.Attributes.SharedCopyEngineCount),
		DecoderCount:        int64(gpu.Attributes.SharedDecoderCount),
		EncoderCount:        int64(gpu.Attributes.SharedEncoderCount),
		JpegCount:           int64(gpu.Attributes.SharedJpegCount),
		OfaCount:            int64(gpu.Attributes.SharedOfaCount),
		MemoryBytes:         int64(gpu.MemoryBytes),
	}

	var mslices []int
	for i := 0; i < int(gpu.Attributes.MemorySlices.Size); i++ {
		mslices = append(mslices, int(gpu.Attributes.MemorySlices.Start)+i)
	}

	mslicesSet := NamedResourcesSet[int]{
		Name:  "memory-slices",
		Items: mslices,
	}

	group := []NamedResourcesGroup{
		{
			Name:       fmt.Sprintf("gpu-%v-shared-resources", gpu.Index),
			Quantities: quantities.ToNamedResourcesQuantities(),
			IntSets:    []NamedResourcesSet[int]{mslicesSet},
		},
	}

	return group
}

// getResources gets the set of shared resources consumed by the MIG device.
func (mig *MigInfo) getResources() []NamedResourcesGroup {
	quantities := consumableQuantities{
		MultiprocessorCount: int64(mig.GIProfileInfo.MultiprocessorCount),
		CopyEngineCount:     int64(mig.GIProfileInfo.CopyEngineCount),
		DecoderCount:        int64(mig.GIProfileInfo.DecoderCount),
		EncoderCount:        int64(mig.GIProfileInfo.EncoderCount),
		JpegCount:           int64(mig.GIProfileInfo.JpegCount),
		OfaCount:            int64(mig.GIProfileInfo.OfaCount),
		MemoryBytes:         int64(mig.GIProfileInfo.MemorySizeMB * 1024 * 1024),
	}

	var mslices []int
	for i := 0; i < int(mig.MemorySlices.Size); i++ {
		mslices = append(mslices, int(mig.MemorySlices.Start)+i)
	}

	mslicesSet := NamedResourcesSet[int]{
		Name:  "memory-slices",
		Items: mslices,
	}

	group := []NamedResourcesGroup{
		{
			Name:       fmt.Sprintf("gpu-%v-shared-resources", mig.Parent.Index),
			Quantities: quantities.ToNamedResourcesQuantities(),
			IntSets:    []NamedResourcesSet[int]{mslicesSet},
		},
	}
	return group
}

// ToNamedResourcesQuantities converts consumableQuantities to a list of NamedResourcesQuantities.
func (c *consumableQuantities) ToNamedResourcesQuantities() []NamedResourcesQuantity {
	quantities := []NamedResourcesQuantity{
		{
			Name:  "multiprocessors",
			Value: resource.NewQuantity(c.MultiprocessorCount, resource.BinarySI),
		},
		{
			Name:  "copy-engines",
			Value: resource.NewQuantity(c.CopyEngineCount, resource.BinarySI),
		},
		{
			Name:  "decoders",
			Value: resource.NewQuantity(c.DecoderCount, resource.BinarySI),
		},
		{
			Name:  "encoders",
			Value: resource.NewQuantity(c.EncoderCount, resource.BinarySI),
		},
		{
			Name:  "jpeg-engines",
			Value: resource.NewQuantity(c.JpegCount, resource.BinarySI),
		},
		{
			Name:  "ofa-engines",
			Value: resource.NewQuantity(c.OfaCount, resource.BinarySI),
		},
		{
			Name:  "memory",
			Value: resource.NewQuantity(c.MemoryBytes, resource.BinarySI),
		},
	}
	return quantities
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
