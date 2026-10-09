package router

import (
	"github.com/open-mrp/apikit/endpoint"
)

type Registry struct {
	groups []endpoint.APIEndpointGroup
}

func NewRegistry() *Registry {
	return &Registry{
		groups: make([]endpoint.APIEndpointGroup, 0),
	}
}

func (r *Registry) RegisterGroup(group *endpoint.APIEndpointGroup) {
	r.groups = append(r.groups, *group)
}

func (r *Registry) RegisterEndpoints(router *Router) {
	for _, group := range r.groups {
		for _, endpointer := range group.Endpoints {
			router.HandleEndpoint(endpointer.GetMethod(), endpointer.GetRoute(), endpointer.GetHandler(), endpointer.IsPublic())
		}
	}
}
