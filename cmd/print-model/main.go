package main

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml/mock/dgxa100"
	resourceapi "k8s.io/api/resource/v1alpha2"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	nvdevicelib "github.com/klueska/nvk8s-resourcemodel/pkg/nvdevice"
)

// Main queries the list of allocatable devices and prints them as a kubernetes
// structure resource model.
func main() {
	// Instantiate an instance of a mock dgxa100 server and build a nvDeviceLib
	// from it. The nvDeviceLib is then used to populate the list of allocatable
	// devices from this mock server using standard NVML calls.
	l := nvdevicelib.New(dgxa100.New())

	// Get the full list of allocatable devices from the server.
	allocatable, err := l.GetAllocatableDevices()
	if err != nil {
		klog.Fatalf("Error getAllocatableDevices: %v", err)
	}

	// Build a structured resource model from the list of allocatable devices.
	instances := allocatable.ToNamedResourceInstances()
	model := resourceapi.ResourceModel{
		NamedResources: &resourceapi.NamedResourcesResources{Instances: instances},
	}

	// Print the structured resource model as yaml.
	modelYaml, err := yaml.Marshal(model)
	if err != nil {
		klog.Fatalf("Error marshaling resource model to yaml: %v", err)
	}
	fmt.Printf("%v", string(modelYaml))
}
