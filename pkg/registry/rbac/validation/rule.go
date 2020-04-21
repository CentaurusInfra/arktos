/*
Copyright 2016 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"k8s.io/klog"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
)

type AuthorizationRuleResolver interface {
	// GetRoleReferenceRules attempts to resolve the role reference of a RoleBinding or ClusterRoleBinding.  The passed namespace should be the namepsace
	// of the role binding, the empty string if a cluster role binding.
	GetRoleReferenceRules(roleRef rbacv1.RoleRef, namespace string) ([]rbacv1.PolicyRule, error)

	// RulesFor returns the list of rules that apply to a given user in a given namespace and error.  If an error is returned, the slice of
	// PolicyRules may not be complete, but it contains all retrievable rules.  This is done because policy rules are purely additive and policy determinations
	// can be made on the basis of those rules that are found.
	RulesFor(user user.Info, namespace string) ([]rbacv1.PolicyRule, error)

	// VisitRulesFor invokes visitor() with each rule that applies to a given user in a given namespace, and each error encountered resolving those rules.
	// If visitor() returns false, visiting is short-circuited.
	VisitRulesFor(user user.Info, namespace string, visitor func(source fmt.Stringer, rule *rbacv1.PolicyRule, err error) bool)
}

// ConfirmNoEscalation determines if the roles for a given user in a given namespace encompass the provided role.
func ConfirmNoEscalation(ctx context.Context, ruleResolver AuthorizationRuleResolver, rules []rbacv1.PolicyRule) error {
	ruleResolutionErrors := []error{}

	user, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return fmt.Errorf("no user on context")
	}
	namespace, _ := genericapirequest.NamespaceFrom(ctx)

	ownerRules, err := ruleResolver.RulesFor(user, namespace)
	if err != nil {
		// As per AuthorizationRuleResolver contract, this may return a non fatal error with an incomplete list of policies. Log the error and continue.
		klog.V(1).Infof("non-fatal error getting local rules for %v: %v", user, err)
		ruleResolutionErrors = append(ruleResolutionErrors, err)
	}

	ownerRightsCover, missingRights := Covers(ownerRules, rules)
	if !ownerRightsCover {
		compactMissingRights := missingRights
		if compact, err := CompactRules(missingRights); err == nil {
			compactMissingRights = compact
		}

		missingDescriptions := sets.NewString()
		for _, missing := range compactMissingRights {
			missingDescriptions.Insert(rbacv1helpers.CompactString(missing))
		}

		msg := fmt.Sprintf("user %q (groups=%q) is attempting to grant RBAC permissions not currently held:\n%s", user.GetName(), user.GetGroups(), strings.Join(missingDescriptions.List(), "\n"))
		if len(ruleResolutionErrors) > 0 {
			msg = msg + fmt.Sprintf("; resolution errors: %v", ruleResolutionErrors)
		}

		return errors.New(msg)
	}
	return nil
}

type DefaultRuleResolver struct {
	roleGetter               RoleGetter
	roleBindingLister        RoleBindingLister
	clusterRoleGetter        ClusterRoleGetter
	clusterRoleBindingLister ClusterRoleBindingLister
}

func NewDefaultRuleResolver(roleGetter RoleGetter, roleBindingLister RoleBindingLister, clusterRoleGetter ClusterRoleGetter, clusterRoleBindingLister ClusterRoleBindingLister) *DefaultRuleResolver {
	return &DefaultRuleResolver{roleGetter, roleBindingLister, clusterRoleGetter, clusterRoleBindingLister}
}

type RoleGetter interface {
	GetRole(namespace, name string) (*rbacv1.Role, error)
	GetRoleWithMultiTenancy(tenant, namespace, name string) (*rbacv1.Role, error)
}

type RoleBindingLister interface {
	ListRoleBindings(namespace string) ([]*rbacv1.RoleBinding, error)
	ListRoleBindingsWithMultiTenancy(tenant, namespace string) ([]*rbacv1.RoleBinding, error)
}

type ClusterRoleGetter interface {
	GetClusterRole(name string) (*rbacv1.ClusterRole, error)
	GetClusterRoleWithMultiTenancy(tenant, name string) (*rbacv1.ClusterRole, error)
}

type ClusterRoleBindingLister interface {
	ListClusterRoleBindings() ([]*rbacv1.ClusterRoleBinding, error)
	ListClusterRoleBindingsWithMultiTenancy(tenant string) ([]*rbacv1.ClusterRoleBinding, error)
}

func (r *DefaultRuleResolver) RulesFor(user user.Info, namespace string) ([]rbacv1.PolicyRule, error) {
	visitor := &ruleAccumulator{}
	r.VisitRulesFor(user, namespace, visitor.visit)
	return visitor.rules, utilerrors.NewAggregate(visitor.errors)
}

type ruleAccumulator struct {
	rules  []rbacv1.PolicyRule
	errors []error
}

func (r *ruleAccumulator) visit(source fmt.Stringer, rule *rbacv1.PolicyRule, err error) bool {
	if rule != nil {
		r.rules = append(r.rules, *rule)
	}
	if err != nil {
		r.errors = append(r.errors, err)
	}
	return true
}

func describeSubject(s *rbacv1.Subject, bindingNamespace string) string {
	switch s.Kind {
	case rbacv1.ServiceAccountKind:
		if len(s.Namespace) > 0 {
			return fmt.Sprintf("%s %q", s.Kind, s.Name+"/"+s.Namespace)
		}
		return fmt.Sprintf("%s %q", s.Kind, s.Name+"/"+bindingNamespace)
	default:
		return fmt.Sprintf("%s %q", s.Kind, s.Name)
	}
}

type clusterRoleBindingDescriber struct {
	binding *rbacv1.ClusterRoleBinding
	subject *rbacv1.Subject
}

func (d *clusterRoleBindingDescriber) String() string {
	return fmt.Sprintf("ClusterRoleBinding %q of %s %q to %s",
		d.binding.Name,
		d.binding.RoleRef.Kind,
		d.binding.RoleRef.Name,
		describeSubject(d.subject, ""),
	)
}

type roleBindingDescriber struct {
	binding *rbacv1.RoleBinding
	subject *rbacv1.Subject
}

func (d *roleBindingDescriber) String() string {
	return fmt.Sprintf("RoleBinding %q of %s %q to %s",
		d.binding.Name+"/"+d.binding.Namespace,
		d.binding.RoleRef.Kind,
		d.binding.RoleRef.Name,
		describeSubject(d.subject, d.binding.Namespace),
	)
}

func (r *DefaultRuleResolver) VisitRulesFor(user user.Info, namespace string, visitor func(source fmt.Stringer, rule *rbacv1.PolicyRule, err error) bool) {

	userTenant := user.GetTenant()

	var targetClusterRoleBindings []*rbacv1.ClusterRoleBinding
	// REMOVE userTenant empty when bearer token tenant
	// validation is done: https://github.com/futurewei-cloud/arktos/issues/38
	// in rbac_test.go integration tests are using bearer tokens without tenants
	targetTenants := []string{"", metav1.TenantSystem}
	if userTenant != metav1.TenantSystem && userTenant != "" {
		targetTenants = append(targetTenants, userTenant)
	}
	for _, tenant := range targetTenants {
		if bindings, err := r.clusterRoleBindingLister.ListClusterRoleBindingsWithMultiTenancy(tenant); err != nil {
			if !visitor(nil, nil, err) {
				return
			}
		} else {
			targetClusterRoleBindings = append(targetClusterRoleBindings, bindings...)
		}
	}

	sourceDescriber := &clusterRoleBindingDescriber{}
	for _, clusterRoleBinding := range targetClusterRoleBindings {
		subjectIndex, applies := appliesTo(user, clusterRoleBinding.Subjects, "")
		if !applies {
			continue
		}
		rules, err := r.GetRoleReferenceRulesWithMultiTenancy(clusterRoleBinding.RoleRef, clusterRoleBinding.Tenant, "")
		if err != nil {
			if !visitor(nil, nil, err) {
				return
			}
			continue
		}
		sourceDescriber.binding = clusterRoleBinding
		sourceDescriber.subject = &clusterRoleBinding.Subjects[subjectIndex]
		for i := range rules {
			if !visitor(sourceDescriber, &rules[i], nil) {
				return
			}
		}
	}

	if len(namespace) > 0 {
		// get role bindings from userTenat, system tenant and default tenant
		var targetRoleBindings []*rbacv1.RoleBinding
		targetTenants := []string{metav1.TenantDefault, metav1.TenantSystem}
		if userTenant != metav1.TenantSystem && userTenant != metav1.TenantDefault {
			targetTenants = append(targetTenants, userTenant)
		}
		for _, tenant := range targetTenants {
			if bindings, err := r.roleBindingLister.ListRoleBindingsWithMultiTenancy(tenant, namespace); err != nil {
				if !visitor(nil, nil, err) {
					return
				}
			} else {
				targetRoleBindings = append(targetRoleBindings, bindings...)
			}
		}

		if len(targetRoleBindings) == 0 {
			return
		}
		sourceDescriber := &roleBindingDescriber{}
		for _, roleBinding := range targetRoleBindings {

			subjectIndex, applies := appliesTo(user, roleBinding.Subjects, namespace)
			if !applies {
				continue
			}
			rules, err := r.GetRoleReferenceRulesWithMultiTenancy(roleBinding.RoleRef, roleBinding.Tenant, namespace)
			if err != nil {
				if !visitor(nil, nil, err) {
					return
				}
				continue
			}
			sourceDescriber.binding = roleBinding
			sourceDescriber.subject = &roleBinding.Subjects[subjectIndex]
			for i := range rules {
				if !visitor(sourceDescriber, &rules[i], nil) {
					return
				}
			}
		}
	}
}

// GetRoleReferenceRulesWithMultiTenancy attempts to resolve the RoleBinding or ClusterRoleBinding.
func (r *DefaultRuleResolver) GetRoleReferenceRulesWithMultiTenancy(roleRef rbacv1.RoleRef, tenant, bindingNamespace string) ([]rbacv1.PolicyRule, error) {
	switch roleRef.Kind {
	case "Role":
		role, err := r.roleGetter.GetRoleWithMultiTenancy(tenant, bindingNamespace, roleRef.Name)
		if err != nil {
			return nil, err
		}
		return role.Rules, nil

	case "ClusterRole":
		clusterRole, err := r.clusterRoleGetter.GetClusterRoleWithMultiTenancy(tenant, roleRef.Name)
		if err != nil {
			return nil, err
		}
		return clusterRole.Rules, nil

	default:
		return nil, fmt.Errorf("unsupported role reference kind: %q", roleRef.Kind)
	}
}

// GetRoleReferenceRules attempts to resolve the RoleBinding or ClusterRoleBinding.
func (r *DefaultRuleResolver) GetRoleReferenceRules(roleRef rbacv1.RoleRef, bindingNamespace string) ([]rbacv1.PolicyRule, error) {
	switch roleRef.Kind {
	case "Role":
		role, err := r.roleGetter.GetRole(bindingNamespace, roleRef.Name)
		if err != nil {
			return nil, err
		}
		return role.Rules, nil

	case "ClusterRole":
		clusterRole, err := r.clusterRoleGetter.GetClusterRole(roleRef.Name)
		if err != nil {
			return nil, err
		}
		return clusterRole.Rules, nil

	default:
		return nil, fmt.Errorf("unsupported role reference kind: %q", roleRef.Kind)
	}
}

// appliesTo returns whether any of the bindingSubjects applies to the specified subject,
// and if true, the index of the first subject that applies
func appliesTo(user user.Info, bindingSubjects []rbacv1.Subject, namespace string) (int, bool) {
	for i, bindingSubject := range bindingSubjects {
		if appliesToUser(user, bindingSubject, namespace) {
			return i, true
		}
	}
	return 0, false
}

func appliesToUser(user user.Info, subject rbacv1.Subject, namespace string) bool {
	switch subject.Kind {
	case rbacv1.UserKind:
		return user.GetName() == subject.Name

	case rbacv1.GroupKind:
		return has(user.GetGroups(), subject.Name)

	case rbacv1.ServiceAccountKind:
		// default the namespace to namespace we're working in if its available.  This allows rolebindings that reference
		// SAs in th local namespace to avoid having to qualify them.
		saNamespace := namespace
		if len(subject.Namespace) > 0 {
			saNamespace = subject.Namespace
		}
		if len(saNamespace) == 0 {
			return false
		}
		// use a more efficient comparison for RBAC checking
		return serviceaccount.MatchesUsername(saNamespace, subject.Name, user.GetName())
	default:
		return false
	}
}

// NewTestRuleResolver returns a rule resolver from lists of role objects.
func NewTestRuleResolver(roles []*rbacv1.Role, roleBindings []*rbacv1.RoleBinding, clusterRoles []*rbacv1.ClusterRole, clusterRoleBindings []*rbacv1.ClusterRoleBinding) (AuthorizationRuleResolver, *StaticRoles) {
	r := StaticRoles{
		roles:               roles,
		roleBindings:        roleBindings,
		clusterRoles:        clusterRoles,
		clusterRoleBindings: clusterRoleBindings,
	}
	return newMockRuleResolver(&r), &r
}

func newMockRuleResolver(r *StaticRoles) AuthorizationRuleResolver {
	return NewDefaultRuleResolver(r, r, r, r)
}

// StaticRoles is a rule resolver that resolves from lists of role objects.
type StaticRoles struct {
	roles               []*rbacv1.Role
	roleBindings        []*rbacv1.RoleBinding
	clusterRoles        []*rbacv1.ClusterRole
	clusterRoleBindings []*rbacv1.ClusterRoleBinding
}

func (r *StaticRoles) GetRole(namespace, name string) (*rbacv1.Role, error) {
	if len(namespace) == 0 {
		return nil, errors.New("must provide namespace when getting role")
	}
	for _, role := range r.roles {
		if role.Namespace == namespace && role.Name == name {
			return role, nil
		}
	}
	return nil, errors.New("role not found")
}

func (r *StaticRoles) GetRoleWithMultiTenancy(tenant, namespace, name string) (*rbacv1.Role, error) {
	if len(namespace) == 0 {
		return nil, errors.New("must provide namespace when getting role")
	}
	if len(tenant) == 0 {
		return nil, errors.New("must provide namespace when getting role")
	}
	for _, role := range r.roles {
		if role.Namespace == namespace && role.Name == name && role.Tenant == tenant {
			return role, nil
		}
	}
	return nil, errors.New("role not found")
}

func (r *StaticRoles) GetClusterRole(name string) (*rbacv1.ClusterRole, error) {
	for _, clusterRole := range r.clusterRoles {
		if clusterRole.Name == name {
			return clusterRole, nil
		}
	}
	return nil, errors.New("clusterrole not found")
}

func (r *StaticRoles) GetClusterRoleWithMultiTenancy(tenant, name string) (*rbacv1.ClusterRole, error) {
	for _, clusterRole := range r.clusterRoles {
		if clusterRole.Name == name && clusterRole.Tenant == tenant{
			return clusterRole, nil
		}
	}
	return nil, errors.New("clusterrole not found")
}

func (r *StaticRoles) ListRoleBindings(namespace string) ([]*rbacv1.RoleBinding, error) {
	if len(namespace) == 0 {
		return nil, errors.New("must provide namespace when listing role bindings")
	}

	roleBindingList := []*rbacv1.RoleBinding{}
	for _, roleBinding := range r.roleBindings {
		if roleBinding.Namespace != namespace {
			continue
		}
		// TODO(ericchiang): need to implement label selectors?
		roleBindingList = append(roleBindingList, roleBinding)
	}
	return roleBindingList, nil
}

func (r *StaticRoles) ListRoleBindingsWithMultiTenancy(tenant, namespace string) ([]*rbacv1.RoleBinding, error) {
	if len(namespace) == 0 {
		return nil, errors.New("must provide namespace when listing role bindings")
	}

	var roleBindingList []*rbacv1.RoleBinding
	for _, roleBinding := range r.roleBindings {
		if roleBinding.Namespace != namespace || roleBinding.Tenant != tenant {
			continue
		}
		// TODO(ericchiang): need to implement label selectors?
		roleBindingList = append(roleBindingList, roleBinding)
	}
	return roleBindingList, nil
}

func (r *StaticRoles) ListClusterRoleBindings() ([]*rbacv1.ClusterRoleBinding, error) {
	return r.clusterRoleBindings, nil
}

func (r *StaticRoles) ListClusterRoleBindingsWithMultiTenancy(tenant string) ([]*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBindingList := []*rbacv1.ClusterRoleBinding{}
	for _, clusterRoleBinding := range r.clusterRoleBindings {
		if clusterRoleBinding.Tenant != tenant {
			continue
		}
		// TODO(ericchiang): need to implement label selectors?
		clusterRoleBindingList = append(clusterRoleBindingList, clusterRoleBinding)
	}
	return clusterRoleBindingList, nil
}
