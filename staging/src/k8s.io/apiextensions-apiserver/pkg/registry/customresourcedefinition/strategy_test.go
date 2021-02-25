/*
Copyright 2017 The Kubernetes Authors.

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

package customresourcedefinition

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsfeatures "k8s.io/apiextensions-apiserver/pkg/features"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/endpoints/request"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
)

func TestDropDisableFieldsCustomResourceDefinition(t *testing.T) {
	t.Log("testing unversioned validation..")
	for _, validationEnabled := range []bool{true, false} {
		crdWithUnversionedValidation := func() *apiextensions.CustomResourceDefinition {
			// crd with non-versioned validation
			return &apiextensions.CustomResourceDefinition{
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Validation: &apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensions.JSONSchemaProps{},
					},
				},
			}
		}
		crdWithoutUnversionedValidation := func() *apiextensions.CustomResourceDefinition {
			// crd with non-versioned validation
			return &apiextensions.CustomResourceDefinition{
				Spec: apiextensions.CustomResourceDefinitionSpec{},
			}
		}
		crdInfos := []struct {
			name            string
			hasCRValidation bool
			crd             func() *apiextensions.CustomResourceDefinition
		}{
			{
				name:            "has unversioned validation",
				hasCRValidation: true,
				crd:             crdWithUnversionedValidation,
			},
			{
				name:            "doesn't have unversioned validation",
				hasCRValidation: false,
				crd:             crdWithoutUnversionedValidation,
			},
			{
				name:            "nil",
				hasCRValidation: false,
				crd:             func() *apiextensions.CustomResourceDefinition { return nil },
			},
		}
		for _, oldCRDInfo := range crdInfos {
			for _, newCRDInfo := range crdInfos {
				oldCRDHasValidation, oldCRD := oldCRDInfo.hasCRValidation, oldCRDInfo.crd()
				newCRDHasValidation, newCRD := newCRDInfo.hasCRValidation, newCRDInfo.crd()
				if newCRD == nil {
					continue
				}
				t.Run(fmt.Sprintf("validation feature enabled=%v, old CRD %v, new CRD %v", validationEnabled, oldCRDInfo.name, newCRDInfo.name),
					func(t *testing.T) {
						defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, apiextensionsfeatures.CustomResourceValidation, validationEnabled)()
						var oldCRDSpec *apiextensions.CustomResourceDefinitionSpec
						if oldCRD != nil {
							oldCRDSpec = &oldCRD.Spec
						}
						dropDisabledFields(&newCRD.Spec, oldCRDSpec)
						// old CRD should never be changed
						if !reflect.DeepEqual(oldCRD, oldCRDInfo.crd()) {
							t.Errorf("old crd changed: %v", diff.ObjectReflectDiff(oldCRD, oldCRDInfo.crd()))
						}
						switch {
						case validationEnabled || oldCRDHasValidation:
							if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
								t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
							}
						case newCRDHasValidation:
							if reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
								t.Errorf("new crd was not changed")
							}
							if !reflect.DeepEqual(newCRD, crdWithoutUnversionedValidation()) {
								t.Errorf("new crd had unversioned validation: %v", diff.ObjectReflectDiff(newCRD, crdWithoutUnversionedValidation()))
							}
						default:
							if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
								t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
							}
						}
					},
				)
			}
		}
	}

	t.Log("testing unversioned subresources...")
	for _, validationEnabled := range []bool{true, false} {
		crdWithUnversionedSubresources := func() *apiextensions.CustomResourceDefinition {
			// crd with unversioned subresources
			return &apiextensions.CustomResourceDefinition{
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Subresources: &apiextensions.CustomResourceSubresources{},
				},
			}
		}
		crdWithoutUnversionedSubresources := func() *apiextensions.CustomResourceDefinition {
			// crd without unversioned subresources
			return &apiextensions.CustomResourceDefinition{
				Spec: apiextensions.CustomResourceDefinitionSpec{},
			}
		}
		crdInfos := []struct {
			name              string
			hasCRSubresources bool
			crd               func() *apiextensions.CustomResourceDefinition
		}{
			{
				name:              "has unversioned subresources",
				hasCRSubresources: true,
				crd:               crdWithUnversionedSubresources,
			},
			{
				name:              "doesn't have unversioned subresources",
				hasCRSubresources: false,
				crd:               crdWithoutUnversionedSubresources,
			},
			{
				name:              "nil",
				hasCRSubresources: false,
				crd:               func() *apiextensions.CustomResourceDefinition { return nil },
			},
		}
		for _, oldCRDInfo := range crdInfos {
			for _, newCRDInfo := range crdInfos {
				oldCRDHasSubresources, oldCRD := oldCRDInfo.hasCRSubresources, oldCRDInfo.crd()
				newCRDHasSubresources, newCRD := newCRDInfo.hasCRSubresources, newCRDInfo.crd()
				if newCRD == nil {
					continue
				}
				t.Run(fmt.Sprintf("subresources feature enabled=%v, old CRD %v, new CRD %v", validationEnabled, oldCRDInfo.name, newCRDInfo.name),
					func(t *testing.T) {
						defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, apiextensionsfeatures.CustomResourceSubresources, validationEnabled)()
						var oldCRDSpec *apiextensions.CustomResourceDefinitionSpec
						if oldCRD != nil {
							oldCRDSpec = &oldCRD.Spec
						}
						dropDisabledFields(&newCRD.Spec, oldCRDSpec)
						// old CRD should never be changed
						if !reflect.DeepEqual(oldCRD, oldCRDInfo.crd()) {
							t.Errorf("old crd changed: %v", diff.ObjectReflectDiff(oldCRD, oldCRDInfo.crd()))
						}
						switch {
						case validationEnabled || oldCRDHasSubresources:
							if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
								t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
							}
						case newCRDHasSubresources:
							if reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
								t.Errorf("new crd was not changed")
							}
							if !reflect.DeepEqual(newCRD, crdWithoutUnversionedSubresources()) {
								t.Errorf("new crd had unversioned subresources: %v", diff.ObjectReflectDiff(newCRD, crdWithoutUnversionedSubresources()))
							}
						default:
							if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
								t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
							}
						}
					},
				)
			}
		}
	}

	t.Log("testing versioned validation..")
	for _, conversionEnabled := range []bool{true, false} {
		for _, validationEnabled := range []bool{true, false} {
			crdWithVersionedValidation := func() *apiextensions.CustomResourceDefinition {
				// crd with versioned validation
				return &apiextensions.CustomResourceDefinition{
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Versions: []apiextensions.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &apiextensions.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensions.JSONSchemaProps{},
								},
							},
						},
					},
				}
			}
			crdWithoutVersionedValidation := func() *apiextensions.CustomResourceDefinition {
				// crd with versioned validation
				return &apiextensions.CustomResourceDefinition{
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Versions: []apiextensions.CustomResourceDefinitionVersion{
							{
								Name: "v1",
							},
						},
					},
				}
			}
			crdInfos := []struct {
				name            string
				hasCRValidation bool
				crd             func() *apiextensions.CustomResourceDefinition
			}{
				{
					name:            "has versioned validation",
					hasCRValidation: true,
					crd:             crdWithVersionedValidation,
				},
				{
					name:            "doesn't have versioned validation",
					hasCRValidation: false,
					crd:             crdWithoutVersionedValidation,
				},
				{
					name:            "nil",
					hasCRValidation: false,
					crd:             func() *apiextensions.CustomResourceDefinition { return nil },
				},
			}
			for _, oldCRDInfo := range crdInfos {
				for _, newCRDInfo := range crdInfos {
					oldCRDHasValidation, oldCRD := oldCRDInfo.hasCRValidation, oldCRDInfo.crd()
					newCRDHasValidation, newCRD := newCRDInfo.hasCRValidation, newCRDInfo.crd()
					if newCRD == nil {
						continue
					}
					t.Run(fmt.Sprintf("validation feature enabled=%v, old CRD %v, new CRD %v", validationEnabled, oldCRDInfo.name, newCRDInfo.name),
						func(t *testing.T) {
							defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, apiextensionsfeatures.CustomResourceValidation, validationEnabled)()
							defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, apiextensionsfeatures.CustomResourceWebhookConversion, conversionEnabled)()
							var oldCRDSpec *apiextensions.CustomResourceDefinitionSpec
							if oldCRD != nil {
								oldCRDSpec = &oldCRD.Spec
							}
							dropDisabledFields(&newCRD.Spec, oldCRDSpec)
							// old CRD should never be changed
							if !reflect.DeepEqual(oldCRD, oldCRDInfo.crd()) {
								t.Errorf("old crd changed: %v", diff.ObjectReflectDiff(oldCRD, oldCRDInfo.crd()))
							}
							switch {
							case (conversionEnabled && validationEnabled) || oldCRDHasValidation:
								if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
									t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
								}
							case !conversionEnabled && !oldCRDHasValidation:
								if !reflect.DeepEqual(newCRD, crdWithoutVersionedValidation()) {
									t.Errorf("new crd was not changed")
								}
							case newCRDHasValidation:
								if reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
									t.Errorf("new crd was not changed")
								}
								if !reflect.DeepEqual(newCRD, crdWithoutVersionedValidation()) {
									t.Errorf("new crd had unversioned validation: %v", diff.ObjectReflectDiff(newCRD, crdWithoutVersionedValidation()))
								}
							default:
								if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
									t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
								}
							}
						},
					)
				}
			}
		}
	}

	t.Log("testing versioned subresources w/ conversion enabled..")
	for _, conversionEnabled := range []bool{true, false} {
		for _, validationEnabled := range []bool{true, false} {
			crdWithVersionedSubresources := func() *apiextensions.CustomResourceDefinition {
				// crd with versioned subresources
				return &apiextensions.CustomResourceDefinition{
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Versions: []apiextensions.CustomResourceDefinitionVersion{
							{
								Name:         "v1",
								Subresources: &apiextensions.CustomResourceSubresources{},
							},
						},
					},
				}
			}
			crdWithoutVersionedSubresources := func() *apiextensions.CustomResourceDefinition {
				// crd without versioned subresources
				return &apiextensions.CustomResourceDefinition{
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Versions: []apiextensions.CustomResourceDefinitionVersion{
							{
								Name: "v1",
							},
						},
					},
				}
			}
			crdInfos := []struct {
				name              string
				hasCRSubresources bool
				crd               func() *apiextensions.CustomResourceDefinition
			}{
				{
					name:              "has versioned subresources",
					hasCRSubresources: true,
					crd:               crdWithVersionedSubresources,
				},
				{
					name:              "doesn't have versioned subresources",
					hasCRSubresources: false,
					crd:               crdWithoutVersionedSubresources,
				},
				{
					name:              "nil",
					hasCRSubresources: false,
					crd:               func() *apiextensions.CustomResourceDefinition { return nil },
				},
			}
			for _, oldCRDInfo := range crdInfos {
				for _, newCRDInfo := range crdInfos {
					oldCRDHasSubresources, oldCRD := oldCRDInfo.hasCRSubresources, oldCRDInfo.crd()
					newCRDHasSubresources, newCRD := newCRDInfo.hasCRSubresources, newCRDInfo.crd()
					if newCRD == nil {
						continue
					}
					t.Run(fmt.Sprintf("subresources feature enabled=%v, old CRD %v, new CRD %v", validationEnabled, oldCRDInfo.name, newCRDInfo.name),
						func(t *testing.T) {
							defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, apiextensionsfeatures.CustomResourceSubresources, validationEnabled)()
							defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, apiextensionsfeatures.CustomResourceWebhookConversion, conversionEnabled)()
							var oldCRDSpec *apiextensions.CustomResourceDefinitionSpec
							if oldCRD != nil {
								oldCRDSpec = &oldCRD.Spec
							}
							dropDisabledFields(&newCRD.Spec, oldCRDSpec)
							// old CRD should never be changed
							if !reflect.DeepEqual(oldCRD, oldCRDInfo.crd()) {
								t.Errorf("old crd changed: %v", diff.ObjectReflectDiff(oldCRD, oldCRDInfo.crd()))
							}
							switch {
							case (conversionEnabled && validationEnabled) || oldCRDHasSubresources:
								if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
									t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
								}
							case !conversionEnabled && !oldCRDHasSubresources:
								if !reflect.DeepEqual(newCRD, crdWithoutVersionedSubresources()) {
									t.Errorf("new crd was not changed")
								}
							case newCRDHasSubresources:
								if reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
									t.Errorf("new crd was not changed")
								}
								if !reflect.DeepEqual(newCRD, crdWithoutVersionedSubresources()) {
									t.Errorf("new crd had versioned subresources: %v", diff.ObjectReflectDiff(newCRD, crdWithoutVersionedSubresources()))
								}
							default:
								if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
									t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
								}
							}
						},
					)
				}
			}
		}
	}

	t.Log("testing conversion webhook..")
	for _, validationEnabled := range []bool{true, false} {
		crdWithUnversionedConversionWebhook := func() *apiextensions.CustomResourceDefinition {
			// crd with conversion webhook
			return &apiextensions.CustomResourceDefinition{
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Conversion: &apiextensions.CustomResourceConversion{
						WebhookClientConfig: &apiextensions.WebhookClientConfig{},
					},
				},
			}
		}
		crdWithoutUnversionedConversionWebhook := func() *apiextensions.CustomResourceDefinition {
			// crd with conversion webhook
			return &apiextensions.CustomResourceDefinition{
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Conversion: &apiextensions.CustomResourceConversion{},
				},
			}
		}
		crdInfos := []struct {
			name                   string
			hasCRConversionWebhook bool
			crd                    func() *apiextensions.CustomResourceDefinition
		}{
			{
				name:                   "has conversion webhook",
				hasCRConversionWebhook: true,
				crd:                    crdWithUnversionedConversionWebhook,
			},
			{
				name:                   "doesn't have conversion webhook",
				hasCRConversionWebhook: false,
				crd:                    crdWithoutUnversionedConversionWebhook,
			},
			{
				name:                   "nil",
				hasCRConversionWebhook: false,
				crd:                    func() *apiextensions.CustomResourceDefinition { return nil },
			},
		}
		for _, oldCRDInfo := range crdInfos {
			for _, newCRDInfo := range crdInfos {
				oldCRDHasConversionWebhook, oldCRD := oldCRDInfo.hasCRConversionWebhook, oldCRDInfo.crd()
				newCRDHasConversionWebhook, newCRD := newCRDInfo.hasCRConversionWebhook, newCRDInfo.crd()
				if newCRD == nil {
					continue
				}
				t.Run(fmt.Sprintf("subresources feature enabled=%v, old CRD %v, new CRD %v", validationEnabled, oldCRDInfo.name, newCRDInfo.name),
					func(t *testing.T) {
						defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, apiextensionsfeatures.CustomResourceWebhookConversion, validationEnabled)()
						var oldCRDSpec *apiextensions.CustomResourceDefinitionSpec
						if oldCRD != nil {
							oldCRDSpec = &oldCRD.Spec
						}
						dropDisabledFields(&newCRD.Spec, oldCRDSpec)
						// old CRD should never be changed
						if !reflect.DeepEqual(oldCRD, oldCRDInfo.crd()) {
							t.Errorf("old crd changed: %v", diff.ObjectReflectDiff(oldCRD, oldCRDInfo.crd()))
						}
						switch {
						case validationEnabled || oldCRDHasConversionWebhook:
							if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
								t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
							}
						case newCRDHasConversionWebhook:
							if reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
								t.Errorf("new crd was not changed")
							}
							if !reflect.DeepEqual(newCRD, crdWithoutUnversionedConversionWebhook()) {
								t.Errorf("new crd had webhook conversion: %v", diff.ObjectReflectDiff(newCRD, crdWithoutUnversionedConversionWebhook()))
							}
						default:
							if !reflect.DeepEqual(newCRD, newCRDInfo.crd()) {
								t.Errorf("new crd changed: %v", diff.ObjectReflectDiff(newCRD, newCRDInfo.crd()))
							}
						}
					},
				)
			}
		}
	}

}

func strPtr(in string) *string {
	return &in
}

func TestValidateAPIApproval(t *testing.T) {
	okFn := func(t *testing.T, errors field.ErrorList) {
		t.Helper()
		if len(errors) > 0 {
			t.Fatal(errors)
		}
	}

	tests := []struct {
		name string

		version            string
		group              string
		annotationValue    string
		oldAnnotationValue *string
		validateError      func(t *testing.T, errors field.ErrorList)
	}{
		{
			name:            "ignore v1beta1",
			version:         "v1beta1",
			group:           "sigs.k8s.io",
			annotationValue: "invalid",
			validateError:   okFn,
		},
		{
			name:            "ignore non-k8s group",
			version:         "v1",
			group:           "other.io",
			annotationValue: "invalid",
			validateError:   okFn,
		},
		{
			name:            "invalid annotation create",
			version:         "v1",
			group:           "sigs.k8s.io",
			annotationValue: "invalid",
			validateError: func(t *testing.T, errors field.ErrorList) {
				t.Helper()
				if e, a := `metadata.annotations[api-approved.kubernetes.io]: Invalid value: "invalid": protected groups must have approval annotation "api-approved.kubernetes.io" with either a URL or a reason starting with "unapproved", see https://github.com/kubernetes/enhancements/pull/1111`, errors.ToAggregate().Error(); e != a {
					t.Fatal(errors)
				}
			},
		},
		{
			name:               "invalid annotation update",
			version:            "v1",
			group:              "sigs.k8s.io",
			annotationValue:    "invalid",
			oldAnnotationValue: strPtr("invalid"),
			validateError:      okFn,
		},
		{
			name:               "invalid annotation to missing",
			version:            "v1",
			group:              "sigs.k8s.io",
			annotationValue:    "",
			oldAnnotationValue: strPtr("invalid"),
			validateError: func(t *testing.T, errors field.ErrorList) {
				t.Helper()
				if e, a := `metadata.annotations[api-approved.kubernetes.io]: Required value: protected groups must have approval annotation "api-approved.kubernetes.io", see https://github.com/kubernetes/enhancements/pull/1111`, errors.ToAggregate().Error(); e != a {
					t.Fatal(errors)
				}
			},
		},
		{
			name:               "missing to invalid annotation",
			version:            "v1",
			group:              "sigs.k8s.io",
			annotationValue:    "invalid",
			oldAnnotationValue: strPtr(""),
			validateError: func(t *testing.T, errors field.ErrorList) {
				t.Helper()
				if e, a := `metadata.annotations[api-approved.kubernetes.io]: Invalid value: "invalid": protected groups must have approval annotation "api-approved.kubernetes.io" with either a URL or a reason starting with "unapproved", see https://github.com/kubernetes/enhancements/pull/1111`, errors.ToAggregate().Error(); e != a {
					t.Fatal(errors)
				}
			},
		},
		{
			name:            "missing annotation",
			version:         "v1",
			group:           "sigs.k8s.io",
			annotationValue: "",
			validateError: func(t *testing.T, errors field.ErrorList) {
				t.Helper()
				if e, a := `metadata.annotations[api-approved.kubernetes.io]: Required value: protected groups must have approval annotation "api-approved.kubernetes.io", see https://github.com/kubernetes/enhancements/pull/1111`, errors.ToAggregate().Error(); e != a {
					t.Fatal(errors)
				}
			},
		},
		{
			name:               "missing annotation update",
			version:            "v1",
			group:              "sigs.k8s.io",
			annotationValue:    "",
			oldAnnotationValue: strPtr(""),
			validateError:      okFn,
		},
		{
			name:            "url",
			version:         "v1",
			group:           "sigs.k8s.io",
			annotationValue: "https://github.com/kubernetes/kubernetes/pull/79724",
			validateError:   okFn,
		},
		{
			name:            "unapproved",
			version:         "v1",
			group:           "sigs.k8s.io",
			annotationValue: "unapproved, other reason",
			validateError:   okFn,
		},
		{
			name:            "next version validates",
			version:         "v2",
			group:           "sigs.k8s.io",
			annotationValue: "invalid",
			validateError: func(t *testing.T, errors field.ErrorList) {
				t.Helper()
				if e, a := `metadata.annotations[api-approved.kubernetes.io]: Invalid value: "invalid": protected groups must have approval annotation "api-approved.kubernetes.io" with either a URL or a reason starting with "unapproved", see https://github.com/kubernetes/enhancements/pull/1111`, errors.ToAggregate().Error(); e != a {
					t.Fatal(errors)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := request.WithRequestInfo(context.TODO(), &request.RequestInfo{APIVersion: test.version})
			crd := &apiextensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{v1beta1.KubeAPIApprovedAnnotation: test.annotationValue}},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Group: test.group,
				},
			}
			var oldCRD *apiextensions.CustomResourceDefinition
			if test.oldAnnotationValue != nil {
				oldCRD = &apiextensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Annotations: map[string]string{v1beta1.KubeAPIApprovedAnnotation: *test.oldAnnotationValue}},
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Group: test.group,
					},
				}
			}

			actual := validateAPIApproval(ctx, crd, oldCRD)
			test.validateError(t, actual)
		})
	}
}
