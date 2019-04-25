// +build !ignore_autogenerated

/*
Copyright 2019 The arhat.dev Authors.

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

// Code generated by kube-openapi-gen. DO NOT EDIT.

// This file was autogenerated by openapi-gen. Do not edit it manually!

package v1alpha1

import (
	spec "github.com/go-openapi/spec"
	common "k8s.io/kube-openapi/pkg/common"
)

func GetOpenAPIDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	return map[string]common.OpenAPIDefinition{
		"./pkg/apis/aranya/v1alpha1/.CertInfo":         schema_apis_aranya_v1alpha1__CertInfo(ref),
		"./pkg/apis/aranya/v1alpha1/.EdgeDevice":       schema_apis_aranya_v1alpha1__EdgeDevice(ref),
		"./pkg/apis/aranya/v1alpha1/.EdgeDeviceSpec":   schema_apis_aranya_v1alpha1__EdgeDeviceSpec(ref),
		"./pkg/apis/aranya/v1alpha1/.EdgeDeviceStatus": schema_apis_aranya_v1alpha1__EdgeDeviceStatus(ref),
	}
}

func schema_apis_aranya_v1alpha1__CertInfo(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "CertInfo is the extra location info for device",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"country": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"state": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"locality": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"org": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
					"orgUnit": {
						SchemaProps: spec.SchemaProps{
							Type:   []string{"string"},
							Format: "",
						},
					},
				},
			},
		},
	}
}

func schema_apis_aranya_v1alpha1__EdgeDevice(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "EdgeDevice is the Schema for the edgedevices API",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"kind": {
						SchemaProps: spec.SchemaProps{
							Description: "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("./pkg/apis/aranya/v1alpha1/.EdgeDeviceSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("./pkg/apis/aranya/v1alpha1/.EdgeDeviceStatus"),
						},
					},
				},
			},
		},
		Dependencies: []string{
			"./pkg/apis/aranya/v1alpha1/.EdgeDeviceSpec", "./pkg/apis/aranya/v1alpha1/.EdgeDeviceStatus", "k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"},
	}
}

func schema_apis_aranya_v1alpha1__EdgeDeviceSpec(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "EdgeDeviceSpec defines the desired state of EdgeDevice",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"certInfo": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("./pkg/apis/aranya/v1alpha1/.CertInfo"),
						},
					},
					"connectivity": {
						SchemaProps: spec.SchemaProps{
							Description: "Connectivity designate the method by which this device connect to aranya server",
							Ref:         ref("./pkg/apis/aranya/v1alpha1/.Connectivity"),
						},
					},
				},
			},
		},
		Dependencies: []string{
			"./pkg/apis/aranya/v1alpha1/.CertInfo", "./pkg/apis/aranya/v1alpha1/.Connectivity"},
	}
}

func schema_apis_aranya_v1alpha1__EdgeDeviceStatus(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "EdgeDeviceStatus defines the observed state of EdgeDevice",
				Type:        []string{"object"},
			},
		},
	}
}
