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

package registration

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/specterops/bloodhound/cmd/api/src/api"
	"github.com/specterops/bloodhound/cmd/api/src/api/middleware"
	"github.com/specterops/bloodhound/cmd/api/src/api/router"
	"github.com/specterops/bloodhound/cmd/api/src/api/static"
	v2 "github.com/specterops/bloodhound/cmd/api/src/api/v2"
	"github.com/specterops/bloodhound/cmd/api/src/auth"
	"github.com/specterops/bloodhound/cmd/api/src/config"
	"github.com/specterops/bloodhound/cmd/api/src/database"
	"github.com/specterops/bloodhound/cmd/api/src/queries"
	"github.com/specterops/bloodhound/cmd/api/src/services/upload"
	"github.com/specterops/bloodhound/packages/go/cache"
	"github.com/specterops/dawgs/graph"
)

func RegisterFossGlobalMiddleware(routerInst *router.Router, cfg config.Configuration, identityResolver auth.IdentityResolver, authenticator api.Authenticator) {
	// Set up the middleware stack
	routerInst.UsePrerouting(middleware.ContextMiddleware)
	routerInst.UsePrerouting(middleware.CORSMiddleware())

	// Set up logging. This must be done after ContextMiddleware is initialized so the context can be accessed in the log logic
	if cfg.EnableAPILogging {
		routerInst.UsePrerouting(middleware.LoggingMiddleware(identityResolver))
	}

	routerInst.UsePostrouting(
		middleware.PanicHandler,
		middleware.AuthMiddleware(authenticator),
		middleware.CompressionMiddleware,
	)
}

func RegisterFossRoutes(
	routerInst *router.Router,
	cfg config.Configuration,
	rdms database.Database,
	graphDB *graph.DatabaseSwitch,
	graphQuery queries.Graph,
	apiCache cache.Cache,
	collectorManifests config.CollectorManifests,
	authenticator api.Authenticator,
	authorizer auth.Authorizer,
	ingestSchema upload.IngestSchema,
) {
	router.With(func() mux.MiddlewareFunc {
		return middleware.DefaultRateLimitMiddleware(rdms)
	},
		// Health Endpoint
		routerInst.GET("/health", func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusOK)
		}),

		// Redirect root resource to the UI
		routerInst.GET("/", func(response http.ResponseWriter, request *http.Request) {
			http.Redirect(response, request, "/ui", http.StatusMovedPermanently)
		}),

		// Static asset handling for the UI
		routerInst.PathPrefix("/ui", static.AssetHandler),
	)

	var resources = v2.NewResources(rdms, graphDB, cfg, apiCache, graphQuery, collectorManifests, authorizer, authenticator, ingestSchema)
	NewV2API(resources, routerInst)
}
