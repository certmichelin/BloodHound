// Copyright 2025 Specter Ops, Inc.
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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gorilla/mux"
	"github.com/specterops/bloodhound/cmd/api/src/api"
	"github.com/specterops/bloodhound/cmd/api/src/database"
	"github.com/specterops/bloodhound/cmd/api/src/model"
)

const (
	CustomNodeKindParameter = "kind_name"
)

var validColorString = regexp.MustCompile("^#([a-fA-F0-9]{6}|[a-fA-F0-9]{3})$")

func (s *Resources) GetCustomNodeKinds(response http.ResponseWriter, request *http.Request) {
	if kinds, err := s.DB.GetCustomNodeKinds(request.Context()); err != nil {
		api.HandleDatabaseError(request, response, err)
	} else {
		api.WriteBasicResponse(request.Context(), kinds, http.StatusOK, response)
	}
}

func (s *Resources) GetCustomNodeKind(response http.ResponseWriter, request *http.Request) {
	var (
		paramId = mux.Vars(request)[CustomNodeKindParameter]
	)

	if kind, err := s.DB.GetCustomNodeKind(request.Context(), paramId); err != nil {
		api.HandleDatabaseError(request, response, err)
	} else {
		api.WriteBasicResponse(request.Context(), kind, http.StatusOK, response)
	}
}

type CreateCustomNodeRequest struct {
	CustomTypes map[string]model.CustomNodeKindConfig `json:"custom_types"`
}

func validateCreateCustomNodeRequest(customNodeKindRequest CreateCustomNodeRequest) error {
	for key, config := range customNodeKindRequest.CustomTypes {
		if key == "" {
			return fmt.Errorf("custom_types contains an entry with an empty string as a key. please remove or replace the empty key")
		} else if err := validateConfig(config); err != nil {
			return err
		}
	}

	return nil
}

func validateConfig(config model.CustomNodeKindConfig) error {
	if config.Icon.Type != "font-awesome" {
		return fmt.Errorf("invalid icon type. only Font Awesome icons are supported")
	} else if !validColorString.MatchString(config.Icon.Color) && config.Icon.Color != "" {
		return fmt.Errorf("icon color must be a valid hexadecimal color string starting with '#' followed by 3 or 6 hex digits")
	}

	return nil
}

func (s *Resources) CreateCustomNodeKind(response http.ResponseWriter, request *http.Request) {
	var (
		customNodeKindRequest CreateCustomNodeRequest
	)

	if err := json.NewDecoder(request.Body).Decode(&customNodeKindRequest); err != nil {
		api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusBadRequest, api.ErrorResponsePayloadUnmarshalError, request), response)
	} else if err := validateCreateCustomNodeRequest(customNodeKindRequest); err != nil {
		api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusBadRequest, fmt.Sprintf("%s: %s", api.ErrorResponseCodeBadRequest, err), request), response)
	} else if kinds, err := s.DB.CreateCustomNodeKinds(request.Context(), convertCreateCustomNodeRequest(customNodeKindRequest)); errors.Is(err, database.ErrDuplicateCustomNodeKindName) {
		api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusConflict, fmt.Sprintf("%s: duplicate kind name", api.ErrorResponseConflict), request), response)
	} else if err != nil {
		api.HandleDatabaseError(request, response, err)
	} else {
		api.WriteBasicResponse(request.Context(), kinds, http.StatusCreated, response)
	}
}

func convertCreateCustomNodeRequest(request CreateCustomNodeRequest) []model.CustomNodeKind {
	var customNodeKinds []model.CustomNodeKind

	for key, val := range request.CustomTypes {
		customNodeKinds = append(customNodeKinds, model.CustomNodeKind{
			KindName: key,
			Config:   assignColorDefault(val),
		})
	}

	return customNodeKinds
}

func assignColorDefault(config model.CustomNodeKindConfig) model.CustomNodeKindConfig {
	if config.Icon.Color == "" {
		config.Icon.Color = "#FFFFFF"
	}

	return config
}

type UpdateCustomNodeKindRequest struct {
	Config model.CustomNodeKindConfig `json:"config"`
}

func (s *Resources) UpdateCustomNodeKind(response http.ResponseWriter, request *http.Request) {
	var (
		paramId               = mux.Vars(request)[CustomNodeKindParameter]
		customNodeKindRequest UpdateCustomNodeKindRequest
	)

	if err := json.NewDecoder(request.Body).Decode(&customNodeKindRequest); err != nil {
		api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusBadRequest, api.ErrorResponsePayloadUnmarshalError, request), response)
	} else if err := validateConfig(customNodeKindRequest.Config); err != nil {
		api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusBadRequest, fmt.Sprintf("%s: %s", api.ErrorResponseCodeBadRequest, err), request), response)
	} else if kind, err := s.DB.UpdateCustomNodeKind(request.Context(), model.CustomNodeKind{KindName: paramId, Config: assignColorDefault(customNodeKindRequest.Config)}); err != nil {
		api.HandleDatabaseError(request, response, err)
	} else {
		api.WriteBasicResponse(request.Context(), kind, http.StatusOK, response)
	}
}

func (s *Resources) DeleteCustomNodeKind(response http.ResponseWriter, request *http.Request) {
	var (
		paramId = mux.Vars(request)[CustomNodeKindParameter]
	)

	if err := s.DB.DeleteCustomNodeKind(request.Context(), paramId); err != nil {
		api.HandleDatabaseError(request, response, err)
	} else {
		response.WriteHeader(http.StatusOK)
	}
}
