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

package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/specterops/bloodhound/cmd/api/src/auth"
	"github.com/specterops/bloodhound/cmd/api/src/config"
	"github.com/specterops/bloodhound/cmd/api/src/database"
	"github.com/specterops/bloodhound/cmd/api/src/database/types/null"
	"github.com/specterops/bloodhound/cmd/api/src/migrations"
	"github.com/specterops/bloodhound/cmd/api/src/model"
	"github.com/specterops/bloodhound/cmd/api/src/model/appcfg"
	"github.com/specterops/dawgs/graph"
)

const (
	DefaultServerShutdownTimeout = time.Minute
	ContentSecurityPolicy        = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:;"
)

func NewDaemonContext(parentCtx context.Context) context.Context {
	daemonContext, doneFunc := context.WithCancel(parentCtx)

	go func() {
		defer doneFunc()

		// Shutdown on SIGINT/SIGTERM
		signalChannel := make(chan os.Signal, 1)
		signal.Notify(signalChannel, syscall.SIGTERM)
		signal.Notify(signalChannel, syscall.SIGINT)

		// Wait for a signal from the OS
		<-signalChannel
	}()

	return daemonContext
}

// MigrateGraph runs migrations for the graph database
func MigrateGraph(ctx context.Context, db graph.Database, schema graph.Schema) error {
	return migrations.NewGraphMigrator(db).Migrate(ctx, schema)
}

// MigrateDB runs database migrations on PG
func MigrateDB(ctx context.Context, cfg config.Configuration, db database.Database) error {
	if err := db.Migrate(ctx); err != nil {
		return err
	}

	if hasInstallation, err := db.HasInstallation(ctx); err != nil {
		return err
	} else if hasInstallation {
		return nil
	}

	return CreateDefaultAdmin(ctx, cfg, db)
}

func CreateDefaultAdmin(ctx context.Context, cfg config.Configuration, db database.Database) error {
	var secretDigester = cfg.Crypto.Argon2.NewDigester()

	if roles, err := db.GetAllRoles(ctx, "", model.SQLFilter{}); err != nil {
		return fmt.Errorf("error while attempting to fetch user roles: %w", err)
	} else if secretDigest, err := secretDigester.Digest(cfg.DefaultAdmin.Password); err != nil {
		return fmt.Errorf("error while attempting to digest secret for user: %w", err)
	} else if adminRole, found := roles.FindByName(auth.RoleAdministrator); !found {
		return fmt.Errorf("unable to find admin role")
	} else {
		if existingUser, err := db.LookupUser(ctx, cfg.DefaultAdmin.PrincipalName); !errors.Is(err, database.ErrNotFound) && err != nil {
			return fmt.Errorf("unable to lookup existing admin user: %w", err)
		} else if err == nil {
			if err := db.DeleteUser(ctx, existingUser); err != nil {
				return fmt.Errorf("unable to delete exisiting admin user: %s: %w", existingUser.PrincipalName, err)
			}
		}

		var (
			adminUser = model.User{
				Roles: model.Roles{
					adminRole,
				},
				PrincipalName: cfg.DefaultAdmin.PrincipalName,
				EmailAddress:  null.NewString(cfg.DefaultAdmin.EmailAddress, true),
				FirstName:     null.NewString(cfg.DefaultAdmin.FirstName, true),
				LastName:      null.NewString(cfg.DefaultAdmin.LastName, true),
			}

			authSecret = model.AuthSecret{
				Digest:       secretDigest.String(),
				DigestMethod: secretDigester.Method(),
			}
		)

		if cfg.DefaultAdmin.ExpireNow {
			authSecret.ExpiresAt = time.Time{}
		} else {
			authSecret.ExpiresAt = time.Now().Add(appcfg.GetPasswordExpiration(ctx, db))
		}

		if _, err := db.InitializeSecretAuth(ctx, adminUser, authSecret); err != nil {
			return fmt.Errorf("error in database while initalizing auth: %w", err)
		} else {
			passwordMsg := fmt.Sprintf("# Initial Password Set To:    %s    #", cfg.DefaultAdmin.Password)
			paddingString := strings.Repeat(" ", len(passwordMsg)-2)
			borderString := strings.Repeat("#", len(passwordMsg))

			slog.Info(borderString)
			slog.Info(fmt.Sprintf("#%s#", paddingString))
			slog.Info(passwordMsg)
			slog.Info(fmt.Sprintf("#%s#", paddingString))
			slog.Info(borderString)
		}
	}

	return nil
}
