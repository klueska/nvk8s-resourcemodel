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
func toMap[S ~[]E, E namedType](s S) map[string]*E {
	m := make(map[string]*E)
	for i := range s {
		m[s[i].GetName()] = &s[i]
	}
	return m
}

// toList takes any map of pointers to objects with strings as keys and turns it into a slice or objects.
func toList[M map[string]*E, E any](m M) []E {
	s := make([]E, 0, len(m))
	for _, v := range m {
		s = append(s, *v)
	}
	return s
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

	newQuantitiesMap := toMap(g.Quantities)
	for _, q := range other.Quantities {
		if _, exists := newQuantitiesMap[q.Name]; !exists {
			return false, fmt.Errorf("missing %v", q.Name)
		}
		newQuantity := newQuantitiesMap[q.Name].DeepCopy()
		newQuantity.Value.Add(*q.Value)
		newQuantitiesMap[q.Name] = newQuantity
	}

	newIntSetsMap := toMap(g.IntSets)
	for _, s := range other.IntSets {
		if _, exists := newIntSetsMap[s.Name]; !exists {
			return false, fmt.Errorf("missing %v", s.Name)
		}
		newIntSet := newIntSetsMap[s.Name].DeepCopy()
		for _, item := range s.Items {
			if slices.Contains(newIntSet.Items, item) {
				return false, fmt.Errorf("item already in %v: %v", s.Name, item)
			}
			newIntSet.Items = append(newIntSet.Items, item)
		}
		newIntSetsMap[s.Name] = newIntSet
	}

	g.Quantities = toList(newQuantitiesMap)
	g.IntSets = toList(newIntSetsMap)

	return true, nil
}

// Sub subtracts the resources of one NamedResourcesGroup from another.
func (g *NamedResourcesGroup) Sub(other *NamedResourcesGroup) (bool, error) {
	if g.Name != other.Name {
		return false, fmt.Errorf("different group names")
	}

	newQuantitiesMap := toMap(g.Quantities)
	for _, q := range other.Quantities {
		if _, exists := newQuantitiesMap[q.Name]; !exists {
			return false, fmt.Errorf("missing %v", q.Name)
		}
		if newQuantitiesMap[q.Name].Value.Cmp(*q.Value) < 0 {
			return false, nil
		}
		newQuantity := newQuantitiesMap[q.Name].DeepCopy()
		newQuantity.Value.Sub(*q.Value)
		newQuantitiesMap[q.Name] = newQuantity
	}

	newIntSetsMap := toMap(g.IntSets)
	for _, s := range other.IntSets {
		if _, exists := newIntSetsMap[s.Name]; !exists {
			return false, fmt.Errorf("missing %v", s.Name)
		}
		for _, item := range s.Items {
			if !slices.Contains(newIntSetsMap[s.Name].Items, item) {
				return false, nil
			}
		}
		var newInts []int
		for _, item := range newIntSetsMap[s.Name].Items {
			if slices.Contains(s.Items, item) {
				continue
			}
			newInts = append(newInts, item)
		}
		newIntSet := newIntSetsMap[s.Name].DeepCopy()
		newIntSet.Items = newInts
		newIntSetsMap[s.Name] = newIntSet
	}

	g.Quantities = toList(newQuantitiesMap)
	g.IntSets = toList(newIntSetsMap)

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
