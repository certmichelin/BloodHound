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
	"github.com/gorilla/schema"
	"github.com/specterops/bloodhound/cmd/api/src/api"
	"github.com/specterops/bloodhound/cmd/api/src/auth"
	"github.com/specterops/bloodhound/cmd/api/src/config"
	"github.com/specterops/bloodhound/cmd/api/src/database"
	"github.com/specterops/bloodhound/cmd/api/src/database/types/null"
	"github.com/specterops/bloodhound/cmd/api/src/model"
	"github.com/specterops/bloodhound/cmd/api/src/queries"
	"github.com/specterops/bloodhound/cmd/api/src/serde"
	"github.com/specterops/bloodhound/cmd/api/src/services/fs"
	"github.com/specterops/bloodhound/cmd/api/src/services/upload"
	"github.com/specterops/bloodhound/packages/go/cache"
	"github.com/specterops/dawgs/graph"
)

type ListPermissionsResponse struct {
	Permissions model.Permissions `json:"permissions"`
}

type ListRolesResponse struct {
	Roles model.Roles `json:"roles"`
}

type ListUsersResponse struct {
	Users model.Users `json:"users"`
}

type ListTokensResponse struct {
	Tokens model.AuthTokens `json:"tokens"`
}

type SAMLSignOnEndpoint struct {
	Name          string    `json:"name"`
	InitiationURL serde.URL `json:"initiation_url"`
}

type ListSAMLSignOnEndpointsResponse struct {
	Endpoints []SAMLSignOnEndpoint `json:"endpoints"`
}

type ListSAMLProvidersResponse struct {
	SAMLProviders model.SAMLProviders `json:"saml_providers"`
}

type UpdateUserRequest struct {
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	EmailAddress   string     `json:"email_address"`
	Principal      string     `json:"principal"`
	Roles          []int32    `json:"roles"`
	SAMLProviderID string     `json:"saml_provider_id"`
	SSOProviderID  null.Int32 `json:"sso_provider_id"`
	IsDisabled     *bool      `json:"is_disabled,omitempty"`
}

type CreateUserRequest struct {
	UpdateUserRequest
	SetUserSecretRequest
}

type DeleteSAMLProviderResponse struct {
	AffectedUsers model.Users `json:"affected_users"`
}

type SetUserSecretRequest struct {
	CurrentSecret      string `json:"current_secret"`
	Secret             string `json:"secret" validate:"password,length=12,lower=1,upper=1,special=1,numeric=1"`
	NeedsPasswordReset bool   `json:"needs_password_reset"`
}

type CreateUserToken struct {
	TokenName string `json:"token_name"`
	UserID    string `json:"user_id"`
}

type CreateOIDCProviderRequest struct {
	Name     string `json:"name"`
	Issuer   string `json:"issuer"`
	ClientId string `json:"client_id"`
}

// Resources holds the database and configuration dependencies to be passed around the API functions
type Resources struct {
	Decoder                    *schema.Decoder
	DB                         database.Database
	Graph                      graph.Database // TODO: to be phased out in favor of graph queries
	GraphQuery                 queries.Graph
	Config                     config.Configuration
	QueryParameterFilterParser model.QueryParameterFilterParser
	Cache                      cache.Cache
	CollectorManifests         config.CollectorManifests
	Authorizer                 auth.Authorizer
	Authenticator              api.Authenticator
	IngestSchema               upload.IngestSchema
	FileService                fs.Service
}

func NewResources(
	rdms database.Database,
	graphDB *graph.DatabaseSwitch,
	cfg config.Configuration,
	apiCache cache.Cache,
	graphQuery queries.Graph,
	collectorManifests config.CollectorManifests,
	authorizer auth.Authorizer,
	authenticator api.Authenticator,
	ingestSchema upload.IngestSchema,
) Resources {
	return Resources{
		Decoder:                    schema.NewDecoder(),
		DB:                         rdms,
		Graph:                      graphDB, // TODO: to be phased out in favor of graph queries
		GraphQuery:                 graphQuery,
		Config:                     cfg,
		QueryParameterFilterParser: model.NewQueryParameterFilterParser(),
		Cache:                      apiCache,
		CollectorManifests:         collectorManifests,
		Authorizer:                 authorizer,
		Authenticator:              authenticator,
		IngestSchema:               ingestSchema,
		FileService:                &fs.Client{},
	}
}
