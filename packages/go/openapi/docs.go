// Copyright 2024 Specter Ops, Inc.
//
// Licensed under the Apache License, Version 2.0
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	_ "embed"
	"net/http"

	"github.com/specterops/bloodhound/packages/go/headers"
	"github.com/specterops/bloodhound/packages/go/mediatypes"
)

//go:embed doc/openapi.json
var openApiJson []byte

func HttpHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(headers.ContentType.String(), mediatypes.ApplicationJson.String())
	w.WriteHeader(http.StatusOK)
	w.Write(openApiJson)
}
