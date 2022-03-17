//go:build !ignore_autogenerated_runtime
// +build !ignore_autogenerated_runtime

/*
Copyright The Kubernetes Authors.

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

// Code generated by conversion-gen. DO NOT EDIT.

package v1alpha1

import (
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	v1alpha3 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha3"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*DiscoveryHookRequest)(nil), (*v1alpha3.DiscoveryHookRequest)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_DiscoveryHookRequest_To_v1alpha3_DiscoveryHookRequest(a.(*DiscoveryHookRequest), b.(*v1alpha3.DiscoveryHookRequest), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha3.DiscoveryHookRequest)(nil), (*DiscoveryHookRequest)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_DiscoveryHookRequest_To_v1alpha1_DiscoveryHookRequest(a.(*v1alpha3.DiscoveryHookRequest), b.(*DiscoveryHookRequest), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha3.DiscoveryHookResponse)(nil), (*DiscoveryHookResponse)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_DiscoveryHookResponse_To_v1alpha1_DiscoveryHookResponse(a.(*v1alpha3.DiscoveryHookResponse), b.(*DiscoveryHookResponse), scope)
	}); err != nil {
		return err
	}
	if err := s.AddConversionFunc((*DiscoveryHookResponse)(nil), (*v1alpha3.DiscoveryHookResponse)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_DiscoveryHookResponse_To_v1alpha3_DiscoveryHookResponse(a.(*DiscoveryHookResponse), b.(*v1alpha3.DiscoveryHookResponse), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1alpha1_DiscoveryHookRequest_To_v1alpha3_DiscoveryHookRequest(in *DiscoveryHookRequest, out *v1alpha3.DiscoveryHookRequest, s conversion.Scope) error {
	return nil
}

// Convert_v1alpha1_DiscoveryHookRequest_To_v1alpha3_DiscoveryHookRequest is an autogenerated conversion function.
func Convert_v1alpha1_DiscoveryHookRequest_To_v1alpha3_DiscoveryHookRequest(in *DiscoveryHookRequest, out *v1alpha3.DiscoveryHookRequest, s conversion.Scope) error {
	return autoConvert_v1alpha1_DiscoveryHookRequest_To_v1alpha3_DiscoveryHookRequest(in, out, s)
}

func autoConvert_v1alpha3_DiscoveryHookRequest_To_v1alpha1_DiscoveryHookRequest(in *v1alpha3.DiscoveryHookRequest, out *DiscoveryHookRequest, s conversion.Scope) error {
	return nil
}

// Convert_v1alpha3_DiscoveryHookRequest_To_v1alpha1_DiscoveryHookRequest is an autogenerated conversion function.
func Convert_v1alpha3_DiscoveryHookRequest_To_v1alpha1_DiscoveryHookRequest(in *v1alpha3.DiscoveryHookRequest, out *DiscoveryHookRequest, s conversion.Scope) error {
	return autoConvert_v1alpha3_DiscoveryHookRequest_To_v1alpha1_DiscoveryHookRequest(in, out, s)
}

func autoConvert_v1alpha1_DiscoveryHookResponse_To_v1alpha3_DiscoveryHookResponse(in *DiscoveryHookResponse, out *v1alpha3.DiscoveryHookResponse, s conversion.Scope) error {
	// WARNING: in.Status requires manual conversion: does not exist in peer-type
	out.Message = in.Message
	// WARNING: in.Extensions requires manual conversion: does not exist in peer-type
	return nil
}

func autoConvert_v1alpha3_DiscoveryHookResponse_To_v1alpha1_DiscoveryHookResponse(in *v1alpha3.DiscoveryHookResponse, out *DiscoveryHookResponse, s conversion.Scope) error {
	out.Message = in.Message
	return nil
}

// Convert_v1alpha3_DiscoveryHookResponse_To_v1alpha1_DiscoveryHookResponse is an autogenerated conversion function.
func Convert_v1alpha3_DiscoveryHookResponse_To_v1alpha1_DiscoveryHookResponse(in *v1alpha3.DiscoveryHookResponse, out *DiscoveryHookResponse, s conversion.Scope) error {
	return autoConvert_v1alpha3_DiscoveryHookResponse_To_v1alpha1_DiscoveryHookResponse(in, out, s)
}
