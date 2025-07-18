// Copyright 2023 Specter Ops, Inc.
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

package v2

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/specterops/bloodhound/cmd/api/src/api"
	"github.com/specterops/bloodhound/cmd/api/src/auth"
	"github.com/specterops/bloodhound/cmd/api/src/ctx"
	"github.com/specterops/bloodhound/cmd/api/src/model/appcfg"
)

type ListFlagsResponse struct {
	Data []appcfg.FeatureFlag `json:"data"`
}

func (s Resources) GetFlags(response http.ResponseWriter, request *http.Request) {
	if flags, err := s.DB.GetAllFlags(request.Context()); err != nil {
		api.HandleDatabaseError(request, response, err)
	} else {
		api.WriteBasicResponse(request.Context(), flags, http.StatusOK, response)
	}
}

type ToggleFlagResponse struct {
	Enabled bool `json:"enabled"`
}

func (s Resources) ToggleFlag(response http.ResponseWriter, request *http.Request) {
	rawFeatureID := mux.Vars(request)[api.URIPathVariableFeatureID]

	if featureID, err := strconv.ParseInt(rawFeatureID, 10, 32); err != nil {
		api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusBadRequest, api.ErrorResponseDetailsIDMalformed, request), response)
	} else if featureFlag, err := s.DB.GetFlag(request.Context(), int32(featureID)); err != nil {
		api.HandleDatabaseError(request, response, err)
	} else if !featureFlag.UserUpdatable {
		api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusForbidden, fmt.Sprintf("Feature flag %s(%d) is not user updatable.", featureFlag.Key, featureID), request), response)
	} else {
		featureFlag.Enabled = !featureFlag.Enabled

		if err := s.DB.SetFlag(request.Context(), featureFlag); err != nil {
			api.HandleDatabaseError(request, response, err)
		} else {
			// TODO: Cleanup #ADCSFeatureFlag after full launch.
			if featureFlag.Key == appcfg.FeatureAdcs && !featureFlag.Enabled {
				var userId string
				if user, isUser := auth.GetUserFromAuthCtx(ctx.FromRequest(request).AuthCtx); !isUser {
					slog.WarnContext(request.Context(), "encountered request analysis for unknown user, this shouldn't happen")
					userId = "unknown-user-toggle-flag"
				} else {
					userId = user.ID.String()
				}

				if err := s.DB.RequestAnalysis(request.Context(), userId); err != nil {
					api.HandleDatabaseError(request, response, err)
					return
				}
			}
			api.WriteBasicResponse(request.Context(), ToggleFlagResponse{
				Enabled: featureFlag.Enabled,
			}, http.StatusOK, response)
		}
	}
}
