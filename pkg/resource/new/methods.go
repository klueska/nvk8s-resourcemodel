package resource

import (
	"fmt"
	"slices"
)

// namedType is an internal interface to help with creating a generic toMap() function.
type namedType interface {
	GetName() string
}

// toMap takes any slice of objects implementing the namedType interface and turns it into a map with its name as the key.
func ToMap[S ~[]E, E namedType](s S) map[string]*E {
	m := make(map[string]*E)
	for i := range s {
		m[s[i].GetName()] = &s[i]
	}
	return m
}

// GetName returns the name of a NamedResourcesQuantity to implement the namedType interface.
func (q NamedResourcesQuantity) GetName() string {
	return q.Name
}

// GetName returns the name of a NamedResourcesIntSet to implement the namedType interface.
func (s NamedResourcesIntSet) GetName() string {
	return s.Name
}

// GetName returns the name of a NamedResourcesStringSet to implement the namedType interface.
func (s NamedResourcesStringSet) GetName() string {
	return s.Name
}

// GetName returns the name of a NamedResourcesGroup to implement the namedType interface.
func (g NamedResourcesGroup) GetName() string {
	return g.Name
}

// GetName returns the name of a NamedResourcesInstance to implement the namedType interface.
func (i NamedResourcesInstance) GetName() string {
	return i.Name
}

// Add adds the resources of one NamedResourcesGroup to another.
func (g *NamedResourcesGroup) Add(other *NamedResourcesGroup) (bool, error) {
	if g.Name != other.Name {
		return false, fmt.Errorf("different group names")
	}

	quantitiesMap := ToMap(g.Quantities)
	intSetsMap := ToMap(g.IntSets)

	var newQuantities []NamedResourcesQuantity
	for _, q := range other.Quantities {
		if _, exists := quantitiesMap[q.Name]; !exists {
			return false, fmt.Errorf("missing %v", q.Name)
		}
		newValue := quantitiesMap[q.Name].Value.DeepCopy()
		newValue.Add(*q.Value)
		newQuantity := NamedResourcesQuantity{
			Name:  q.Name,
			Value: &newValue,
		}
		newQuantities = append(newQuantities, newQuantity)
	}

	var newIntSets []NamedResourcesIntSet
	for _, s := range other.IntSets {
		if _, exists := intSetsMap[s.Name]; !exists {
			return false, fmt.Errorf("missing %v", s.Name)
		}
		newItems := slices.Clone(intSetsMap[s.Name].Items)
		for _, item := range s.Items {
			if slices.Contains(newItems, item) {
				return false, fmt.Errorf("item already in %v: %v", s.Name, item)
			}
			newItems = append(newItems, item)
		}
		newIntSet := NamedResourcesIntSet{
			Name:  s.Name,
			Items: newItems,
		}
		newIntSets = append(newIntSets, newIntSet)
	}

	g.Quantities = newQuantities
	g.IntSets = newIntSets

	return true, nil
}

// Sub subtracts the resources of one NamedResourcesGroup from another.
func (g *NamedResourcesGroup) Sub(other *NamedResourcesGroup) (bool, error) {
	if g.Name != other.Name {
		return false, fmt.Errorf("different group names")
	}

	quantitiesMap := ToMap(g.Quantities)
	intSetsMap := ToMap(g.IntSets)

	var newQuantities []NamedResourcesQuantity
	for _, q := range other.Quantities {
		if _, exists := quantitiesMap[q.Name]; !exists {
			return false, fmt.Errorf("missing %v", q.Name)
		}
		if quantitiesMap[q.Name].Value.Cmp(*q.Value) < 0 {
			return false, nil
		}
		newValue := quantitiesMap[q.Name].Value.DeepCopy()
		newValue.Sub(*q.Value)
		newQuantity := NamedResourcesQuantity{
			Name:  q.Name,
			Value: &newValue,
		}
		newQuantities = append(newQuantities, newQuantity)
	}

	var newIntSets []NamedResourcesIntSet
	for _, s := range other.IntSets {
		if _, exists := intSetsMap[s.Name]; !exists {
			return false, fmt.Errorf("missing %v", s.Name)
		}
		for _, item := range s.Items {
			if !slices.Contains(intSetsMap[s.Name].Items, item) {
				return false, nil
			}
		}
		var newItems []int
		for _, item := range intSetsMap[s.Name].Items {
			if slices.Contains(s.Items, item) {
				continue
			}
			newItems = append(newItems, item)
		}
		newIntSet := NamedResourcesIntSet{
			Name:  s.Name,
			Items: newItems,
		}
		newIntSets = append(newIntSets, newIntSet)
	}

	g.Quantities = newQuantities
	g.IntSets = newIntSets

	return true, nil
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
func (g *NamedResourcesGroup) addOrReplaceIntSetIfLarger(s *NamedResourcesIntSet) {
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

// addOrReplaceStringSetIfLarger is an internal function to conditionally update StringSets in a NamedResourcesGroup.
func (g *NamedResourcesGroup) addOrReplaceStringSetIfLarger(s *NamedResourcesStringSet) {
	for i := range g.StringSets {
		if s.Name == g.StringSets[i].Name {
			for _, item := range s.Items {
				if !slices.Contains(g.StringSets[i].Items, item) {
					g.StringSets[i].Items = append(g.StringSets[i].Items, item)
				}
			}
			return
		}
	}
	g.StringSets = append(g.StringSets, *s)
}
