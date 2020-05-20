/*
Copyright 2014 The Kubernetes Authors.
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

package serviceaccount

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"gopkg.in/square/go-jose.v2/jwt"
	"k8s.io/klog"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controller"
)

type testGenerator struct {
	Token string
	Err   error
}

var defaultServiceAccountName = "default"

func (t *testGenerator) GenerateToken(sc *jwt.Claims, pc interface{}) (string, error) {
	return t.Token, t.Err
}

// emptySecretReferences is used by a service account without any secrets
func emptySecretReferences() []v1.ObjectReference {
	return []v1.ObjectReference{}
}

// missingSecretReferences is used by a service account that references secrets which do no exist
func missingSecretReferences() []v1.ObjectReference {
	return []v1.ObjectReference{{Name: "missing-secret-1"}}
}

// regularSecretReferences is used by a service account that references secrets which are not ServiceAccountTokens
func regularSecretReferences() []v1.ObjectReference {
	return []v1.ObjectReference{{Name: "regular-secret-1"}}
}

// tokenSecretReferences is used by a service account that references a ServiceAccountToken secret
func tokenSecretReferences() []v1.ObjectReference {
	return []v1.ObjectReference{{Name: "token-secret-1"}}
}

// addTokenSecretReference adds a reference to the ServiceAccountToken that will be created
func addTokenSecretReference(refs []v1.ObjectReference) []v1.ObjectReference {
	return addNamedTokenSecretReference(refs, "default-token-xn8fg")
}

// addNamedTokenSecretReference adds a reference to the named ServiceAccountToken
func addNamedTokenSecretReference(refs []v1.ObjectReference, name string) []v1.ObjectReference {
	return append(refs, v1.ObjectReference{Name: name})
}

// serviceAccount returns a service account with the given secret refs
func serviceAccount(tenant string, secretRefs []v1.ObjectReference) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            defaultServiceAccountName,
			UID:             "12345",
			Namespace:       metav1.NamespaceDefault,
			Tenant:          tenant,
			ResourceVersion: "1",
		},
		Secrets: secretRefs,
	}
}

// updatedServiceAccount returns a service account with the resource version modified
func updatedServiceAccount(tenant string, secretRefs []v1.ObjectReference) *v1.ServiceAccount {
	sa := serviceAccount(tenant, secretRefs)
	sa.ResourceVersion = "2"
	return sa
}

// opaqueSecret returns a persisted non-ServiceAccountToken secret named "regular-secret-1"
func opaqueSecret(tenant string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "regular-secret-1",
			Namespace:       metav1.NamespaceDefault,
			Tenant:          tenant,
			UID:             "23456",
			ResourceVersion: "1",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"mykey": []byte("mydata"),
		},
	}
}

// createdTokenSecret returns the ServiceAccountToken secret posted when creating a new token secret.
// Named "default-token-xn8fg", since that is the first generated name after rand.Seed(1)
func createdTokenSecret(tenant string, overrideName ...string) *v1.Secret {
	return namedCreatedTokenSecret(tenant, "default-token-xn8fg")
}

// namedTokenSecret returns the ServiceAccountToken secret posted when creating a new token secret with the given name.
func namedCreatedTokenSecret(tenant string, name string) *v1.Secret {
	namespaceKey := tenant + "/" + metav1.NamespaceDefault
	if tenant == metav1.TenantSystem {
		namespaceKey = metav1.NamespaceDefault
	}
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			Tenant:    tenant,
			Annotations: map[string]string{
				v1.ServiceAccountNameKey: defaultServiceAccountName,
				v1.ServiceAccountUIDKey:  "12345",
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token":     []byte("ABC"),
			"ca.crt":    []byte("CA Data"),
			"namespace": []byte(namespaceKey),
		},
	}
}

// serviceAccountTokenSecret returns an existing ServiceAccountToken secret named "token-secret-1"
func serviceAccountTokenSecret(tenant string) *v1.Secret {
	namespaceKey := tenant + "/" + metav1.NamespaceDefault
	if tenant == metav1.TenantSystem {
		namespaceKey = metav1.NamespaceDefault
	}
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "token-secret-1",
			Namespace:       metav1.NamespaceDefault,
			Tenant:          tenant,
			UID:             "23456",
			ResourceVersion: "1",
			Annotations: map[string]string{
				v1.ServiceAccountNameKey: defaultServiceAccountName,
				v1.ServiceAccountUIDKey:  "12345",
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token":     []byte("ABC"),
			"ca.crt":    []byte("CA Data"),
			"namespace": []byte(namespaceKey),
		},
	}
}

// serviceAccountTokenSecretWithoutTokenData returns an existing ServiceAccountToken secret that lacks token data
func serviceAccountTokenSecretWithoutTokenData(tenant string) *v1.Secret {
	secret := serviceAccountTokenSecret(tenant)
	delete(secret.Data, v1.ServiceAccountTokenKey)
	return secret
}

// serviceAccountTokenSecretWithoutCAData returns an existing ServiceAccountToken secret that lacks ca data
func serviceAccountTokenSecretWithoutCAData(tenant string) *v1.Secret {
	secret := serviceAccountTokenSecret(tenant)
	delete(secret.Data, v1.ServiceAccountRootCAKey)
	return secret
}

// serviceAccountTokenSecretWithCAData returns an existing ServiceAccountToken secret with the specified ca data
func serviceAccountTokenSecretWithCAData(tenant string, data []byte) *v1.Secret {
	secret := serviceAccountTokenSecret(tenant)
	secret.Data[v1.ServiceAccountRootCAKey] = data
	return secret
}

// serviceAccountTokenSecretWithoutNamespaceData returns an existing ServiceAccountToken secret that lacks namespace data
func serviceAccountTokenSecretWithoutNamespaceData(tenant string) *v1.Secret {
	secret := serviceAccountTokenSecret(tenant)
	delete(secret.Data, v1.ServiceAccountNamespaceKey)
	return secret
}

// serviceAccountTokenSecretWithNamespaceData returns an existing ServiceAccountToken secret with the specified namespace data
func serviceAccountTokenSecretWithNamespaceData(tenant string, data []byte) *v1.Secret {
	secret := serviceAccountTokenSecret(tenant)
	secret.Data[v1.ServiceAccountNamespaceKey] = data
	return secret
}

type reaction struct {
	verb     string
	resource string
	reactor  func(t *testing.T) core.ReactionFunc
}

func TestTokenCreation(t *testing.T) {
	testTokenCreation(t, metav1.NamespaceDefault)
}

func TestTokenCreationWithMultiTenancy(t *testing.T) {
	testTokenCreation(t, "test-te")
}

func testTokenCreation(t *testing.T, tenant string) {
	testcases := map[string]struct {
		ClientObjects []runtime.Object

		IsAsync    bool
		MaxRetries int

		Reactors []reaction

		ExistingServiceAccount *v1.ServiceAccount
		ExistingSecrets        []*v1.Secret

		AddedServiceAccount   *v1.ServiceAccount
		UpdatedServiceAccount *v1.ServiceAccount
		DeletedServiceAccount *v1.ServiceAccount
		AddedSecret           *v1.Secret
		AddedSecretLocal      *v1.Secret
		UpdatedSecret         *v1.Secret
		DeletedSecret         *v1.Secret

		ExpectedActions []core.Action
	}{
		"new serviceaccount with no secrets": {
			ClientObjects: []runtime.Object{serviceAccount(tenant, emptySecretReferences())},

			AddedServiceAccount: serviceAccount(tenant, emptySecretReferences()),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, createdTokenSecret(tenant), tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, serviceAccount(tenant, addTokenSecretReference(emptySecretReferences())), tenant),
			},
		},
		"new serviceaccount with no secrets encountering create error": {
			ClientObjects: []runtime.Object{serviceAccount(tenant, emptySecretReferences())},
			MaxRetries:    10,
			IsAsync:       true,
			Reactors: []reaction{{
				verb:     "create",
				resource: "secrets",
				reactor: func(t *testing.T) core.ReactionFunc {
					i := 0
					return func(core.Action) (bool, runtime.Object, error) {
						i++
						if i < 3 {
							return true, nil, apierrors.NewForbidden(api.Resource("secrets"), "foo", errors.New("No can do"))
						}
						return false, nil, nil
					}
				},
			}},
			AddedServiceAccount: serviceAccount(tenant, emptySecretReferences()),
			ExpectedActions: []core.Action{
				// Attempt 1
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, createdTokenSecret(tenant), tenant),

				// Attempt 2
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, namedCreatedTokenSecret(tenant, "default-token-txhzt"), tenant),

				// Attempt 3
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, namedCreatedTokenSecret(tenant, "default-token-vnmz7"), tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, serviceAccount(tenant, addNamedTokenSecretReference(emptySecretReferences(), "default-token-vnmz7")), tenant),
			},
		},
		"new serviceaccount with no secrets encountering unending create error": {
			ClientObjects: []runtime.Object{serviceAccount(tenant, emptySecretReferences()), createdTokenSecret(tenant)},
			MaxRetries:    2,
			IsAsync:       true,
			Reactors: []reaction{{
				verb:     "create",
				resource: "secrets",
				reactor: func(t *testing.T) core.ReactionFunc {
					return func(core.Action) (bool, runtime.Object, error) {
						return true, nil, apierrors.NewForbidden(api.Resource("secrets"), "foo", errors.New("No can do"))
					}
				},
			}},

			AddedServiceAccount: serviceAccount(tenant, emptySecretReferences()),
			ExpectedActions: []core.Action{
				// Attempt
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, createdTokenSecret(tenant), tenant),
				// Retry 1
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, namedCreatedTokenSecret(tenant, "default-token-txhzt"), tenant),
				// Retry 2
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, namedCreatedTokenSecret(tenant, "default-token-vnmz7"), tenant),
			},
		},
		"new serviceaccount with missing secrets": {
			ClientObjects: []runtime.Object{serviceAccount(tenant, missingSecretReferences())},

			AddedServiceAccount: serviceAccount(tenant, missingSecretReferences()),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, createdTokenSecret(tenant), tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, serviceAccount(tenant, addTokenSecretReference(missingSecretReferences())), tenant),
			},
		},
		"new serviceaccount with missing secrets and a local secret in the cache": {
			ClientObjects: []runtime.Object{serviceAccount(tenant, missingSecretReferences())},

			AddedServiceAccount: serviceAccount(tenant, tokenSecretReferences()),
			AddedSecretLocal:    serviceAccountTokenSecret(tenant),
			ExpectedActions:     []core.Action{},
		},
		"new serviceaccount with non-token secrets": {
			ClientObjects: []runtime.Object{serviceAccount(tenant, regularSecretReferences()), opaqueSecret(tenant)},

			AddedServiceAccount: serviceAccount(tenant, regularSecretReferences()),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, createdTokenSecret(tenant), tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, serviceAccount(tenant, addTokenSecretReference(regularSecretReferences())), tenant),
			},
		},
		"new serviceaccount with token secrets": {
			ClientObjects:   []runtime.Object{serviceAccount(tenant, tokenSecretReferences()), serviceAccountTokenSecret(tenant)},
			ExistingSecrets: []*v1.Secret{serviceAccountTokenSecret(tenant)},

			AddedServiceAccount: serviceAccount(tenant, tokenSecretReferences()),
			ExpectedActions:     []core.Action{},
		},
		"new serviceaccount with no secrets with resource conflict": {
			ClientObjects: []runtime.Object{updatedServiceAccount(tenant, emptySecretReferences()), createdTokenSecret(tenant)},
			IsAsync:       true,
			MaxRetries:    1,

			AddedServiceAccount: serviceAccount(tenant, emptySecretReferences()),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
			},
		},
		"updated serviceaccount with no secrets": {
			ClientObjects: []runtime.Object{serviceAccount(tenant, emptySecretReferences())},

			UpdatedServiceAccount: serviceAccount(tenant, emptySecretReferences()),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, createdTokenSecret(tenant), tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, serviceAccount(tenant, addTokenSecretReference(emptySecretReferences())), tenant),
			},
		},
		"updated serviceaccount with missing secrets": {
			ClientObjects: []runtime.Object{serviceAccount(tenant, missingSecretReferences())},

			UpdatedServiceAccount: serviceAccount(tenant, missingSecretReferences()),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, createdTokenSecret(tenant), tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, serviceAccount(tenant, addTokenSecretReference(missingSecretReferences())), tenant),
			},
		},
		"updated serviceaccount with non-token secrets": {
			ClientObjects: []runtime.Object{serviceAccount(tenant, regularSecretReferences()), opaqueSecret(tenant)},

			UpdatedServiceAccount: serviceAccount(tenant, regularSecretReferences()),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewCreateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, createdTokenSecret(tenant), tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, serviceAccount(tenant, addTokenSecretReference(regularSecretReferences())), tenant),
			},
		},
		"updated serviceaccount with token secrets": {
			ExistingSecrets: []*v1.Secret{serviceAccountTokenSecret(tenant)},

			UpdatedServiceAccount: serviceAccount(tenant, tokenSecretReferences()),
			ExpectedActions:       []core.Action{},
		},
		"updated serviceaccount with no secrets with resource conflict": {
			ClientObjects: []runtime.Object{updatedServiceAccount(tenant, emptySecretReferences())},
			IsAsync:       true,
			MaxRetries:    1,

			UpdatedServiceAccount: serviceAccount(tenant, emptySecretReferences()),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
			},
		},

		"deleted serviceaccount with no secrets": {
			DeletedServiceAccount: serviceAccount(tenant, emptySecretReferences()),
			ExpectedActions:       []core.Action{},
		},
		"deleted serviceaccount with missing secrets": {
			DeletedServiceAccount: serviceAccount(tenant, missingSecretReferences()),
			ExpectedActions:       []core.Action{},
		},
		"deleted serviceaccount with non-token secrets": {
			ClientObjects: []runtime.Object{opaqueSecret(tenant)},

			DeletedServiceAccount: serviceAccount(tenant, regularSecretReferences()),
			ExpectedActions:       []core.Action{},
		},
		"deleted serviceaccount with token secrets": {
			ClientObjects:   []runtime.Object{serviceAccountTokenSecret(tenant)},
			ExistingSecrets: []*v1.Secret{serviceAccountTokenSecret(tenant)},

			DeletedServiceAccount: serviceAccount(tenant, tokenSecretReferences()),
			ExpectedActions: []core.Action{
				core.NewDeleteActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
			},
		},

		"added secret without serviceaccount": {
			ClientObjects: []runtime.Object{serviceAccountTokenSecret(tenant)},

			AddedSecret: serviceAccountTokenSecret(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewDeleteActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
			},
		},
		"added secret with serviceaccount": {
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			AddedSecret:     serviceAccountTokenSecret(tenant),
			ExpectedActions: []core.Action{},
		},
		"added token secret without token data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithoutTokenData(tenant)},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			AddedSecret: serviceAccountTokenSecretWithoutTokenData(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, serviceAccountTokenSecret(tenant), tenant),
			},
		},
		"added token secret without ca data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithoutCAData(tenant)},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			AddedSecret: serviceAccountTokenSecretWithoutCAData(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, serviceAccountTokenSecret(tenant), tenant),
			},
		},
		"added token secret with mismatched ca data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithCAData(tenant, []byte("mismatched"))},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			AddedSecret: serviceAccountTokenSecretWithCAData(tenant, []byte("mismatched")),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, serviceAccountTokenSecret(tenant), tenant),
			},
		},
		"added token secret without namespace data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithoutNamespaceData(tenant)},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			AddedSecret: serviceAccountTokenSecretWithoutNamespaceData(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, serviceAccountTokenSecret(tenant), tenant),
			},
		},
		"added token secret with custom namespace data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithNamespaceData(tenant, []byte("custom"))},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			AddedSecret:     serviceAccountTokenSecretWithNamespaceData(tenant, []byte("custom")),
			ExpectedActions: []core.Action{
				// no update is performed... the custom namespace is preserved
			},
		},

		"updated secret without serviceaccount": {
			ClientObjects: []runtime.Object{serviceAccountTokenSecret(tenant)},

			UpdatedSecret: serviceAccountTokenSecret(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewDeleteActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
			},
		},
		"updated secret with serviceaccount": {
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			UpdatedSecret:   serviceAccountTokenSecret(tenant),
			ExpectedActions: []core.Action{},
		},
		"updated token secret without token data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithoutTokenData(tenant)},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			UpdatedSecret: serviceAccountTokenSecretWithoutTokenData(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, serviceAccountTokenSecret(tenant), tenant),
			},
		},
		"updated token secret without ca data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithoutCAData(tenant)},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			UpdatedSecret: serviceAccountTokenSecretWithoutCAData(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, serviceAccountTokenSecret(tenant), tenant),
			},
		},
		"updated token secret with mismatched ca data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithCAData(tenant, []byte("mismatched"))},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			UpdatedSecret: serviceAccountTokenSecretWithCAData(tenant, []byte("mismatched")),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, serviceAccountTokenSecret(tenant), tenant),
			},
		},
		"updated token secret without namespace data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithoutNamespaceData(tenant)},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			UpdatedSecret: serviceAccountTokenSecretWithoutNamespaceData(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, "token-secret-1", tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, metav1.NamespaceDefault, serviceAccountTokenSecret(tenant), tenant),
			},
		},
		"updated token secret with custom namespace data": {
			ClientObjects:          []runtime.Object{serviceAccountTokenSecretWithNamespaceData(tenant, []byte("custom"))},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			UpdatedSecret:   serviceAccountTokenSecretWithNamespaceData(tenant, []byte("custom")),
			ExpectedActions: []core.Action{
				// no update is performed... the custom namespace is preserved
			},
		},

		"deleted secret without serviceaccount": {
			DeletedSecret:   serviceAccountTokenSecret(tenant),
			ExpectedActions: []core.Action{},
		},
		"deleted secret with serviceaccount with reference": {
			ClientObjects:          []runtime.Object{serviceAccount(tenant, tokenSecretReferences())},
			ExistingServiceAccount: serviceAccount(tenant, tokenSecretReferences()),

			DeletedSecret: serviceAccountTokenSecret(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
				core.NewUpdateActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, serviceAccount(tenant, emptySecretReferences()), tenant),
			},
		},
		"deleted secret with serviceaccount without reference": {
			ExistingServiceAccount: serviceAccount(tenant, emptySecretReferences()),

			DeletedSecret: serviceAccountTokenSecret(tenant),
			ExpectedActions: []core.Action{
				core.NewGetActionWithMultiTenancy(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}, metav1.NamespaceDefault, defaultServiceAccountName, tenant),
			},
		},
	}

	for k, tc := range testcases {
		klog.Infof(k)

		// Re-seed to reset name generation
		utilrand.Seed(1)

		generator := &testGenerator{Token: "ABC"}

		client := fake.NewSimpleClientset(tc.ClientObjects...)
		for _, reactor := range tc.Reactors {
			client.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactor(t))
		}
		informers := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
		secretInformer := informers.Core().V1().Secrets().Informer()
		secrets := secretInformer.GetStore()
		serviceAccounts := informers.Core().V1().ServiceAccounts().Informer().GetStore()
		controller, err := NewTokensController(informers.Core().V1().ServiceAccounts(), informers.Core().V1().Secrets(), client, TokensControllerOptions{TokenGenerator: generator, RootCA: []byte("CA Data"), MaxRetries: tc.MaxRetries})
		if err != nil {
			t.Fatalf("error creating Tokens controller: %v", err)
		}

		if tc.ExistingServiceAccount != nil {
			serviceAccounts.Add(tc.ExistingServiceAccount)
		}
		for _, s := range tc.ExistingSecrets {
			secrets.Add(s)
		}

		if tc.AddedServiceAccount != nil {
			serviceAccounts.Add(tc.AddedServiceAccount)
			controller.queueServiceAccountSync(tc.AddedServiceAccount)
		}
		if tc.UpdatedServiceAccount != nil {
			serviceAccounts.Add(tc.UpdatedServiceAccount)
			controller.queueServiceAccountUpdateSync(nil, tc.UpdatedServiceAccount)
		}
		if tc.DeletedServiceAccount != nil {
			serviceAccounts.Delete(tc.DeletedServiceAccount)
			controller.queueServiceAccountSync(tc.DeletedServiceAccount)
		}
		if tc.AddedSecret != nil {
			secrets.Add(tc.AddedSecret)
			controller.queueSecretSync(tc.AddedSecret)
		}
		if tc.AddedSecretLocal != nil {
			controller.updatedSecrets.Mutation(tc.AddedSecretLocal)
		}
		if tc.UpdatedSecret != nil {
			secrets.Add(tc.UpdatedSecret)
			controller.queueSecretUpdateSync(nil, tc.UpdatedSecret)
		}
		if tc.DeletedSecret != nil {
			secrets.Delete(tc.DeletedSecret)
			controller.queueSecretSync(tc.DeletedSecret)
		}

		// This is the longest we'll wait for async tests
		timeout := time.Now().Add(30 * time.Second)
		waitedForAdditionalActions := false

		for {
			if controller.syncServiceAccountQueue.Len() > 0 {
				controller.syncServiceAccount()
			}
			if controller.syncSecretQueue.Len() > 0 {
				controller.syncSecret()
			}

			// The queues still have things to work on
			if controller.syncServiceAccountQueue.Len() > 0 || controller.syncSecretQueue.Len() > 0 {
				continue
			}

			// If we expect this test to work asynchronously...
			if tc.IsAsync {
				// if we're still missing expected actions within our test timeout
				if len(client.Actions()) < len(tc.ExpectedActions) && time.Now().Before(timeout) {
					// wait for the expected actions (without hotlooping)
					time.Sleep(time.Millisecond)
					continue
				}

				// if we exactly match our expected actions, wait a bit to make sure no other additional actions show up
				if len(client.Actions()) == len(tc.ExpectedActions) && !waitedForAdditionalActions {
					time.Sleep(time.Second)
					waitedForAdditionalActions = true
					continue
				}
			}

			break
		}

		if controller.syncServiceAccountQueue.Len() > 0 {
			t.Errorf("%s: unexpected items in service account queue: %d", k, controller.syncServiceAccountQueue.Len())
		}
		if controller.syncSecretQueue.Len() > 0 {
			t.Errorf("%s: unexpected items in secret queue: %d", k, controller.syncSecretQueue.Len())
		}

		actions := client.Actions()
		for i, action := range actions {
			if len(tc.ExpectedActions) < i+1 {
				t.Errorf("%s: %d unexpected actions: %+v", k, len(actions)-len(tc.ExpectedActions), actions[i:])
				break
			}

			expectedAction := tc.ExpectedActions[i]
			if !reflect.DeepEqual(expectedAction, action) {
				t.Errorf("%s:\nExpected:\n%s\ngot:\n%s", k, spew.Sdump(expectedAction), spew.Sdump(action))
				continue
			}
		}

		if len(tc.ExpectedActions) > len(actions) {
			t.Errorf("%s: %d additional expected actions", k, len(tc.ExpectedActions)-len(actions))
			for _, a := range tc.ExpectedActions[len(actions):] {
				t.Logf("    %+v", a)
			}
		}
	}
}
