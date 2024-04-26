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

// GetName returns the name of a NamedResourcesSharedResource to implement the namedType interface.
func (q NamedResourcesSharedResource) GetName() string {
	return q.Name
}

// GetName returns the name of a NamedResourcesSharedResourceGroup to implement the namedType interface.
func (g NamedResourcesSharedResourceGroup) GetName() string {
	return g.Name
}

// GetName returns the name of a NamedResourcesInstance to implement the namedType interface.
func (i NamedResourcesInstance) GetName() string {
	return i.Name
}

// Add adds the resources of one NamedResourcesSharedResourceGroup to another.
func (g *NamedResourcesSharedResourceGroup) Add(other *NamedResourcesSharedResourceGroup) (bool, error) {
	if g.Name != other.Name {
		return false, fmt.Errorf("different group names")
	}

	newItems := toMap(g.DeepCopy().Items)
	for _, item := range other.Items {
		name := item.Name
		if _, exists := newItems[name]; !exists {
			return false, fmt.Errorf("missing %v", name)
		}
		if item.QuantityValue != nil {
			if newItems[name].QuantityValue == nil {
				return false, fmt.Errorf("mismatched types for %v", name)
			}
			newItems[name].QuantityValue.Add(*item.QuantityValue)
		}
		if item.IntSliceValue != nil {
			if newItems[name].IntSliceValue == nil {
				return false, fmt.Errorf("mismatched types for %v", name)
			}
			for _, i := range item.IntSliceValue.Ints {
				if slices.Contains(newItems[name].IntSliceValue.Ints, i) {
					return false, fmt.Errorf("item already in %v: %v", name, i)
				}
				newItems[name].IntSliceValue.Ints = append(newItems[name].IntSliceValue.Ints, i)
			}
		}
	}
	g.Items = toList(newItems)

	return true, nil
}

// Sub subtracts the resources of one NamedResourcesSharedResourceGroup from another.
func (g *NamedResourcesSharedResourceGroup) Sub(other *NamedResourcesSharedResourceGroup) (bool, error) {
	if g.Name != other.Name {
		return false, fmt.Errorf("different group names")
	}

	newItems := toMap(g.DeepCopy().Items)
	for _, item := range other.Items {
		name := item.Name
		if _, exists := newItems[name]; !exists {
			return false, fmt.Errorf("missing %v", name)
		}
		if item.QuantityValue != nil {
			if newItems[name].QuantityValue == nil {
				return false, fmt.Errorf("mismatched types for %v", name)
			}
			if newItems[name].QuantityValue.Cmp(*item.QuantityValue) < 0 {
				return false, nil
			}
			newItems[name].QuantityValue.Sub(*item.QuantityValue)
		}
		if item.IntSliceValue != nil {
			if newItems[name].IntSliceValue == nil {
				return false, fmt.Errorf("mismatched types for %v", name)
			}
			for _, i := range item.IntSliceValue.Ints {
				if !slices.Contains(newItems[name].IntSliceValue.Ints, i) {
					return false, nil
				}
			}
			var newInts []int64
			for _, i := range newItems[name].IntSliceValue.Ints {
				if slices.Contains(item.IntSliceValue.Ints, i) {
					continue
				}
				newInts = append(newInts, i)
			}
			newItems[name].IntSliceValue.Ints = newInts
		}
	}
	g.Items = toList(newItems)

	return true, nil
}

// addOrReplaceQuantityIfLarger is an internal function to conditionally update Quantities in a NamedResourcesSharedResourceGroup.
func (g *NamedResourcesSharedResourceGroup) addOrReplaceQuantityIfLarger(r *NamedResourcesSharedResource) {
	for i := range g.Items {
		if r.Name == g.Items[i].Name {
			if r.QuantityValue.Cmp(*g.Items[i].QuantityValue) > 0 {
				*g.Items[i].QuantityValue = *r.QuantityValue
			}
			return
		}
	}
	g.Items = append(g.Items, *r)
}

// addOrReplaceIntSliceIfLarger is an internal function to conditionally update IntSlices in a NamedResourcesSharedResourceGroup.
func (g *NamedResourcesSharedResourceGroup) addOrReplaceIntSliceIfLarger(r *NamedResourcesSharedResource) {
	for i := range g.Items {
		if r.Name == g.Items[i].Name {
			newInts := slices.Concat(g.Items[i].IntSliceValue.Ints, r.IntSliceValue.Ints)
			slices.Sort(newInts)
			g.Items[i].IntSliceValue.Ints = slices.Compact(newInts)
			return
		}
	}
	g.Items = append(g.Items, *r)
}

// addOrReplaceStringSliceIfLarger is an internal function to conditionally update StringSlices in a NamedResourcesSharedResourceGroup.
func (g *NamedResourcesSharedResourceGroup) addOrReplaceStringSliceIfLarger(r *NamedResourcesSharedResource) {
	for i := range g.Items {
		if r.Name == g.Items[i].Name {
			newStrings := slices.Concat(g.Items[i].StringSliceValue.Strings, r.StringSliceValue.Strings)
			slices.Sort(newStrings)
			g.Items[i].StringSliceValue.Strings = slices.Compact(newStrings)
			return
		}
	}
	g.Items = append(g.Items, *r)
}
