/*
Copyright 2021 The Kubernetes Authors.

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

package clusterctl

import (
	"fmt"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/internal/goproxy"
)

func Test_resolveReleaseMarker(t *testing.T) {
	clientGoproxy, muxGoproxy, teardownGoproxy := goproxy.NewFakeGoproxy()
	defer teardownGoproxy()

	// setup an handler with fake releases
	muxGoproxy.HandleFunc("/github.com/o/r1/@v/list", func(w http.ResponseWriter, r *http.Request) {
		goproxy.TestMethod(t, r, "GET")
		fmt.Fprint(w, "v1.2.0\n")
		fmt.Fprint(w, "v1.2.1-rc.0\n")
	})
	tests := []struct {
		name          string
		releaseMarker string
		want          string
		wantErr       bool
	}{
		{
			name:          "Invalid url",
			releaseMarker: "github.com/o/doesntexist",
			want:          "",
			wantErr:       true,
		},
		{
			name:          "Get stable release",
			releaseMarker: "go://github.com/o/r1@v1.2",
			want:          "1.2.0",
			wantErr:       false,
		},
		{
			name:          "Get latest release",
			releaseMarker: "go://github.com/o/r1@latest-v1.2",
			want:          "1.2.1-rc.0",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got, err := resolveReleaseMarker(tt.releaseMarker, clientGoproxy)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(got).To(BeEquivalentTo(tt.want))
		})
	}
}
