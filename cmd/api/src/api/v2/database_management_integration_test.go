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

//go:build integration
// +build integration

package v2_test

import (
	"testing"

	"github.com/specterops/bloodhound/lab"
	v2 "github.com/specterops/bloodhound/src/api/v2"
	"github.com/specterops/bloodhound/src/database"
	"github.com/specterops/bloodhound/src/model"
	"github.com/specterops/bloodhound/src/test/lab/fixtures"
	"github.com/specterops/bloodhound/src/test/lab/harnesses"
	"github.com/stretchr/testify/require"
)

func Test_DatabaseManagement_FileUploadHistory(t *testing.T) {
	var (
		harness           = harnesses.NewIntegrationTestHarness(fixtures.BHAdminApiClientFixture)
		userFixture       *lab.Fixture[*model.User]
		fileUploadFixture *lab.Fixture[*model.FileUploadJobs]
	)

	fixtures.TransactionalFixtures(
		func(db *lab.Fixture[*database.BloodhoundDB]) lab.Depender {
			// create a user
			userFixture = fixtures.NewUserFixture(db)
			return userFixture
		},
		func(db *lab.Fixture[*database.BloodhoundDB]) lab.Depender {
			// create some file upload jobs. file upload jobs have a fk constraint on a user
			fileUploadFixture = fixtures.NewFileUploadFixture(db, userFixture)
			return fileUploadFixture
		},
	)

	lab.Pack(harness, fileUploadFixture)

	lab.NewSpec(t, harness).Run(
		lab.TestCase("the endpoint can delete file ingest history", func(assert *require.Assertions, harness *lab.Harness) {
			apiClient, ok := lab.Unpack(harness, fixtures.BHAdminApiClientFixture)
			assert.True(ok)

			err := apiClient.HandleDatabaseManagement(
				v2.DatabaseManagement{
					FileIngestHistory: true,
				})
			assert.Nil(err, "error calling apiClient.HandleDatabaseManagement")

			db, ok := lab.Unpack(harness, fixtures.PostgresFixture)
			assert.True(ok)

			_, numJobs, err := db.GetAllFileUploadJobs(0, 0, "", model.SQLFilter{})
			assert.Nil(err)
			assert.Zero(numJobs)

			// actual, _ := db.DeleteAllFileUploads()
		}),
	)
}

func Test_DatabaseManagement_AssetGroupSelectors(t *testing.T) {
	var (
		harness            = harnesses.NewIntegrationTestHarness(fixtures.BHAdminApiClientFixture)
		assetGroup         *lab.Fixture[*model.AssetGroup]
		assetGroupSelector *lab.Fixture[*model.AssetGroupSelector]
	)

	fixtures.TransactionalFixtures(
		func(db *lab.Fixture[*database.BloodhoundDB]) lab.Depender {
			assetGroup = fixtures.NewAssetGroupFixture(db, "mycoolassetgroup", "customtag", false)
			return assetGroup
		}, func(db *lab.Fixture[*database.BloodhoundDB]) lab.Depender {
			assetGroupSelector = fixtures.NewAssetGroupSelectorFixture(db, assetGroup, "mycoolassetgroupselector", "someobjectid")
			return assetGroupSelector
		},
	)

	// packing `assetGroupSelector` packs all the things it depends on too
	lab.Pack(harness, assetGroupSelector)

	lab.NewSpec(t, harness).Run(
		lab.TestCase("the endpoint can delete asset group selectors", func(assert *require.Assertions, harness *lab.Harness) {
			apiClient, ok := lab.Unpack(harness, fixtures.BHAdminApiClientFixture)
			assert.True(ok)

			selector, ok := lab.Unpack(harness, assetGroupSelector)
			assert.True(ok, "unable to unpack asset group selector")

			err := apiClient.HandleDatabaseManagement(
				v2.DatabaseManagement{
					HighValueSelectors: true,
					AssetGroupId:       int(selector.AssetGroupID),
				})
			assert.Nil(err, "error calling apiClient.HandleDatabaseManagement")

			db, ok := lab.Unpack(harness, fixtures.PostgresFixture)
			assert.True(ok)

			actual, _ := db.GetAssetGroupSelector(selector.ID)
			expected := model.AssetGroupSelector{}
			// when selector is not found in the db, `db.GetAssetGroupSelector` returns an empty struct.
			assert.Equal(expected, actual)
		}),
	)
}
