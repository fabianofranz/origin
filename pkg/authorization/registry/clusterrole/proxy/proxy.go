package proxy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	rolestorage "github.com/openshift/origin/pkg/authorization/registry/role/policybased"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type ClusterRoleStorage struct {
	roleStorage rolestorage.VirtualStorage
}

func NewClusterRoleStorage(clusterPolicyRegistry clusterpolicyregistry.Registry, ruleResolver rulevalidation.AuthorizationRuleResolver) *ClusterRoleStorage {
	simulatedPolicyRegistry := clusterpolicyregistry.NewSimulatedRegistry(clusterPolicyRegistry)

	return &ClusterRoleStorage{
		roleStorage: rolestorage.VirtualStorage{
			PolicyStorage: simulatedPolicyRegistry,

			RuleResolver:   ruleResolver,
			CreateStrategy: roleregistry.ClusterStrategy,
			UpdateStrategy: roleregistry.ClusterStrategy},
	}
}

func (s *ClusterRoleStorage) New() runtime.Object {
	return &authorizationapi.ClusterRole{}
}
func (s *ClusterRoleStorage) NewList() runtime.Object {
	return &authorizationapi.ClusterRoleList{}
}

func (s *ClusterRoleStorage) List(ctx kapi.Context, options *kapi.ListOptions) (runtime.Object, error) {
	ret, err := s.roleStorage.List(ctx, options)
	if ret == nil {
		return nil, err
	}
	return authorizationapi.ToClusterRoleList(ret.(*authorizationapi.RoleList)), err
}

func (s *ClusterRoleStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	ret, err := s.roleStorage.Get(ctx, name)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRole(ret.(*authorizationapi.Role)), err
}
func (s *ClusterRoleStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	ret, err := s.roleStorage.Delete(ctx, name, options)
	if ret == nil {
		return nil, err
	}

	return ret.(*unversioned.Status), err
}

func (s *ClusterRoleStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	clusterObj := obj.(*authorizationapi.ClusterRole)
	convertedObj := authorizationapi.ToRole(clusterObj)

	ret, err := s.roleStorage.Create(ctx, convertedObj)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRole(ret.(*authorizationapi.Role)), err
}

func (s *ClusterRoleStorage) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	clusterObj := obj.(*authorizationapi.ClusterRole)
	convertedObj := authorizationapi.ToRole(clusterObj)

	ret, created, err := s.roleStorage.Update(ctx, convertedObj)
	if ret == nil {
		return nil, created, err
	}

	return authorizationapi.ToClusterRole(ret.(*authorizationapi.Role)), created, err
}

func (m *ClusterRoleStorage) CreateClusterRoleWithEscalation(ctx kapi.Context, obj *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, error) {
	in := authorizationapi.ToRole(obj)
	ret, err := m.roleStorage.CreateRoleWithEscalation(ctx, in)
	return authorizationapi.ToClusterRole(ret), err
}

func (m *ClusterRoleStorage) UpdateClusterRoleWithEscalation(ctx kapi.Context, obj *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, bool, error) {
	in := authorizationapi.ToRole(obj)
	ret, created, err := m.roleStorage.UpdateRoleWithEscalation(ctx, in)
	return authorizationapi.ToClusterRole(ret), created, err
}

func (m *ClusterRoleStorage) CreateRoleWithEscalation(ctx kapi.Context, obj *authorizationapi.Role) (*authorizationapi.Role, error) {
	return m.roleStorage.CreateRoleWithEscalation(ctx, obj)
}

func (m *ClusterRoleStorage) UpdateRoleWithEscalation(ctx kapi.Context, obj *authorizationapi.Role) (*authorizationapi.Role, bool, error) {
	return m.roleStorage.UpdateRoleWithEscalation(ctx, obj)
}
