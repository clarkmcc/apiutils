package auth

import (
	"fmt"
	"strings"
)

const (
	Wildcard = "*"
)

// PermissionRequirement is a permission that is used as a requirement for
// particular resources or endpoints that the caller needs to have in order
// for us to allow the request. Permission requirements never have wildcards
// and are intended to be explicit.
type PermissionRequirement Permission

type Permission struct {
	Namespace string
	Service   string
	Resource  string
	Verb      string
}

func ParsePermissionRequirementOrDie(in string) PermissionRequirement {
	if strings.Contains(in, Wildcard) {
		panic(fmt.Errorf("permission requirements cannot contain '%v' character", Wildcard))
	}
	p, err := ParsePermissionString(in)
	if err != nil {
		panic(err)
	}
	return PermissionRequirement(p)
}

func ParsePermissionString(in string) (Permission, error) {
	parts := strings.Split(in, ".")
	if len(parts) != 4 {
		return Permission{}, fmt.Errorf("expected 4 parts, got %v", len(parts))
	}
	return Permission{parts[0], parts[1], parts[2], parts[3]}, nil
}

// FulfillsRequirement returns true if the provided permission p fulfills the
// permission requirement r.
func (r PermissionRequirement) FulfillsRequirement(p Permission) bool {
	if r.Namespace != p.Namespace && p.Namespace != Wildcard {
		return false
	}
	if r.Service != p.Service && p.Service != Wildcard {
		return false
	}
	if r.Resource != p.Resource && p.Resource != Wildcard {
		return false
	}
	if r.Verb != p.Verb && p.Verb != Wildcard {
		return false
	}
	return true
}

func (r PermissionRequirement) String() string {
	return Permission(r).String()
}

func (r Permission) String() string {
	return strings.Join([]string{
		r.Namespace,
		r.Service,
		r.Resource,
		r.Verb,
	}, ".")
}

type PermissionRequirementGroup []PermissionRequirement

func NewPermissionRequirementGroup(requirements ...string) (out PermissionRequirementGroup) {
	for _, r := range requirements {
		out = append(out, ParsePermissionRequirementOrDie(r))
	}
	return
}
