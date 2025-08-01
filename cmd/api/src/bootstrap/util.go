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
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specterops/bloodhound/cmd/api/src/api/tools"
	"github.com/specterops/bloodhound/cmd/api/src/config"
	"github.com/specterops/bloodhound/packages/go/bhlog"
	"github.com/specterops/bloodhound/packages/go/bhlog/level"
	"github.com/specterops/dawgs"
	"github.com/specterops/dawgs/drivers/neo4j"
	"github.com/specterops/dawgs/drivers/pg"
	"github.com/specterops/dawgs/graph"
	"github.com/specterops/dawgs/util/size"
)

func ensureDirectory(path string) error {
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("unable to create directory %s: %w", path, err)
		}
	}

	return nil
}

// EnsureServerDirectories checks that all required server directories have been set up.
// If they haven't, it attempts to create them. If creation fails, it returns the error.
func EnsureServerDirectories(cfg config.Configuration) error {
	if err := ensureDirectory(cfg.WorkDir); err != nil {
		return err
	}

	if err := ensureDirectory(cfg.TempDirectory()); err != nil {
		return err
	}

	if err := ensureDirectory(cfg.ClientLogDirectory()); err != nil {
		return err
	}

	if err := ensureDirectory(cfg.CollectorsDirectory()); err != nil {
		return err
	}

	return nil
}

// DefaultConfigFilePath returns the location of the config file
func DefaultConfigFilePath() string {
	return "/etc/bhapi/bhapi.json"
}

func ConnectGraph(ctx context.Context, cfg config.Configuration) (*graph.DatabaseSwitch, error) {
	var (
		connectionString string
		pool             *pgxpool.Pool
		err              error
	)

	driverName, err := tools.LookupGraphDriver(ctx, cfg)
	if err != nil {
		return nil, err
	}

	switch driverName {
	case neo4j.DriverName:
		slog.InfoContext(ctx, "Connecting to graph using Neo4j")
		connectionString = cfg.Neo4J.Neo4jConnectionString()

	case pg.DriverName:
		slog.InfoContext(ctx, "Connecting to graph using PostgreSQL")
		connectionString = cfg.Database.PostgreSQLConnectionString()

		pool, err = pg.NewPool(connectionString)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unknown graphdb driver name: %s", driverName)
	}

	if connectionString == "" {
		return nil, fmt.Errorf("graph connection requires a connection url to be set")
	} else if graphDatabase, err := dawgs.Open(ctx, driverName, dawgs.Config{
		GraphQueryMemoryLimit: size.Size(cfg.GraphQueryMemoryLimit) * size.Gibibyte,
		ConnectionString:      connectionString,
		Pool:                  pool,
	}); err != nil {
		return nil, err
	} else {
		return graph.NewDatabaseSwitch(ctx, graphDatabase), nil
	}
}

// InitializeLogging sets up output file logging, and returns errors if any
func InitializeLogging(cfg config.Configuration) error {
	var logLevel = slog.LevelInfo

	if cfg.LogLevel != "" {
		if parsedLevel, err := bhlog.ParseLevel(cfg.LogLevel); err != nil {
			return err
		} else {
			logLevel = parsedLevel
		}
	}

	if cfg.EnableTextLogger {
		bhlog.ConfigureDefaultText(os.Stdout)
	} else {
		bhlog.ConfigureDefaultJSON(os.Stdout)
	}
	level.SetGlobalLevel(logLevel)

	slog.Info("Logging configured")
	return nil
}
