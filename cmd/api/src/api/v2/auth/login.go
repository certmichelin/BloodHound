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

package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/specterops/bloodhound/cmd/api/src/api"
	"github.com/specterops/bloodhound/cmd/api/src/auth"
	"github.com/specterops/bloodhound/cmd/api/src/config"
	"github.com/specterops/bloodhound/cmd/api/src/ctx"
	"github.com/specterops/bloodhound/cmd/api/src/database"
)

type LoginResource struct {
	cfg           config.Configuration
	authenticator api.Authenticator
	db            database.Database
}

// NewLoginResource creates a new LoginResource object
func NewLoginResource(cfg config.Configuration, authenticator api.Authenticator, db database.Database) LoginResource {
	return LoginResource{
		cfg:           cfg,
		authenticator: authenticator,
		db:            db,
	}
}

func (s LoginResource) loginSecret(loginRequest api.LoginRequest, response http.ResponseWriter, request *http.Request) {
	if loginDetails, err := s.authenticator.LoginWithSecret(request.Context(), loginRequest); err != nil {
		if errors.Is(err, api.ErrInvalidAuth) || errors.Is(err, api.ErrNoUserSecret) {
			api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusUnauthorized, api.ErrorResponseDetailsAuthenticationInvalid, request), response)
		} else if errors.Is(err, auth.ErrInvalidOTP) {
			api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusBadRequest, api.ErrorResponseDetailsOTPInvalid, request), response)
		} else if errors.Is(err, api.ErrUserDisabled) {
			api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusForbidden, err.Error(), request), response)
		} else {
			slog.ErrorContext(request.Context(), fmt.Sprintf("Error during authentication for request ID %s: %v", ctx.RequestID(request), err))
			api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusInternalServerError, api.ErrorResponseDetailsInternalServerError, request), response)
		}
	} else {
		api.WriteBasicResponse(request.Context(), api.LoginResponse{
			UserID:       loginDetails.User.ID.String(),
			AuthExpired:  loginDetails.User.AuthSecret.Expired(),
			SessionToken: loginDetails.SessionToken,
		}, http.StatusOK, response)
	}
}

func (s LoginResource) Login(response http.ResponseWriter, request *http.Request) {
	var loginRequest api.LoginRequest
	if err := api.ReadJSONRequestPayloadLimited(&loginRequest, request); err != nil {
		api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusBadRequest, err.Error(), request), response)
	} else {
		// Trim leading and trailing spaces from the username
		loginRequest.Username = strings.TrimSpace(loginRequest.Username)

		if err = s.patchEULAAcceptance(request.Context(), loginRequest.Username); err != nil {
			api.HandleDatabaseError(request, response, err)
		} else {
			switch strings.ToLower(loginRequest.LoginMethod) {
			case auth.ProviderTypeSecret:
				s.loginSecret(loginRequest, response, request)
			default:
				api.WriteErrorResponse(request.Context(), api.BuildErrorResponse(http.StatusBadRequest, fmt.Sprintf("Login method %s is not supported.", loginRequest.LoginMethod), request), response)
			}
		}
	}
}

// EULA Acceptance does not pertain to Bloodhound Community Edition; this flag is used for Bloodhound Enterprise users.
func (s LoginResource) patchEULAAcceptance(ctx context.Context, username string) error {
	if user, err := s.db.LookupUser(ctx, username); err != nil {
		return err
	} else if !user.EULAAccepted {
		user.EULAAccepted = true
		if err = s.db.UpdateUser(ctx, user); err != nil {
			return err
		}
	}

	return nil
}

func (s LoginResource) Logout(response http.ResponseWriter, request *http.Request) {
	bhCtx := ctx.FromRequest(request)
	s.authenticator.Logout(request.Context(), bhCtx.AuthCtx.Session)
	redirectURL := api.URLJoinPath(*bhCtx.Host, api.UserInterfacePath)
	http.Redirect(response, request, redirectURL.String(), http.StatusOK)
}
