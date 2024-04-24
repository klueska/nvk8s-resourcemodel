package nvdevice

import (
	"fmt"

	"github.com/Masterminds/semver"
	resourceapi "k8s.io/api/resource/v1alpha2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

// ToNamedResourceInstances converts a list of AllocatableDevices to a list of NamedResourcesInstances.
func (devices AllocatableDevices) ToNamedResourceInstances() []resourceapi.NamedResourcesInstance {
	var instances []resourceapi.NamedResourcesInstance
	for _, device := range devices {
		var instance *resourceapi.NamedResourcesInstance
		if device.Mig != nil {
			instance = device.Mig.ToNamedResourcesInstance()
		}
		if device.Gpu != nil {
			instance = device.Gpu.ToNamedResourcesInstance()
		}
		if instance != nil {
			instances = append(instances, *instance)
		}
	}
	return instances
}

// ToNamedResourcesInstance converts a GpuInfo object to a NamedResourcesInstance.
func (gpu *GpuInfo) ToNamedResourcesInstance() *resourceapi.NamedResourcesInstance {
	instance := &resourceapi.NamedResourcesInstance{
		Name: fmt.Sprintf("gpu-%v", gpu.Index),
		Attributes: []resourceapi.NamedResourcesAttribute{
			{
				Name: "minor",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					IntValue: ptr.To(int64(gpu.Minor)),
				},
			},
			{
				Name: "index",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					IntValue: ptr.To(int64(gpu.Index)),
				},
			},
			{
				Name: "uuid",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					StringValue: &gpu.UUID,
				},
			},
			{
				Name: "memory",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					QuantityValue: resource.NewQuantity(int64(gpu.MemoryBytes), resource.BinarySI),
				},
			},
			{
				Name: "product-name",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					StringValue: &gpu.ProductName,
				},
			},
			{
				Name: "brand",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					StringValue: &gpu.Brand,
				},
			},
			{
				Name: "architecture",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					StringValue: &gpu.Architecture,
				},
			},
			{
				Name: "cuda-compute-capability",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					VersionValue: ptr.To(semver.MustParse(gpu.CudaComputeCapability).String()),
				},
			},
			{
				Name: "driver-version",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					VersionValue: ptr.To(semver.MustParse(gpu.DriverVersion).String()),
				},
			},
			{
				Name: "cuda-driver-version",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					VersionValue: ptr.To(semver.MustParse(gpu.CudaDriverVersion).String()),
				},
			},
			{
				Name: "mig-capable",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					BoolValue: &gpu.MigCapable,
				},
			},
			{
				Name: "mig-enabled",
				NamedResourcesAttributeValue: resourceapi.NamedResourcesAttributeValue{
					BoolValue: &gpu.MigEnabled,
				},
			},
		},
	}

	return instance
}

// ToNamedResourcesInstance converts a MigInfo object to a NamedResourcesInstance.
// TODO: This is currently unimplemented.
func (mig *MigInfo) ToNamedResourcesInstance() *resourceapi.NamedResourcesInstance {
	return nil
}
