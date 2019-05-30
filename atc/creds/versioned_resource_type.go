package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
)

type VersionedResourceType struct {
	atc.VersionedResourceType

	Source Source
}

type VersionedResourceTypes []VersionedResourceType

func NewVersionedResourceTypes(variables Variables, rawTypes atc.VersionedResourceTypes) VersionedResourceTypes {
	var types VersionedResourceTypes
	for _, t := range rawTypes {
		types = append(types, VersionedResourceType{
			VersionedResourceType: t,
			Source:                NewSource(variables, t.Source),
		})
	}

	return types
}

func (types VersionedResourceTypes) Lookup(name string) (VersionedResourceType, bool) {
	for _, t := range types {
		if t.Name == name {
			return t, true
		}
	}

	return VersionedResourceType{}, false
}

func (types VersionedResourceTypes) Without(name string) VersionedResourceTypes {
	newTypes := VersionedResourceTypes{}

	for _, t := range types {
		if t.Name != name {
			newTypes = append(newTypes, t)
		}
	}

	return newTypes
}

func (types VersionedResourceTypes) DetermineUnderlyingTypeName(typeName string) string {
	resourceTypesMap := make(map[string]creds.VersionedResourceType)
	for _, resourceType := range types {
		resourceTypesMap[resourceType.Name] = resourceType
	}
	underlyingTypeName := typeName
	underlyingType, ok := resourceTypesMap[underlyingTypeName]
	for ok {
		underlyingTypeName = underlyingType.Type
		underlyingType, ok = resourceTypesMap[underlyingTypeName]
		delete(resourceTypesMap, underlyingTypeName)
	}
	return underlyingTypeName
}
