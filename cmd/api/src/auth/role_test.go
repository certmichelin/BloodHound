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

package auth_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/specterops/bloodhound/cmd/api/src/api"
	v2 "github.com/specterops/bloodhound/cmd/api/src/api/v2"
	"github.com/specterops/bloodhound/cmd/api/src/auth"
	"github.com/specterops/bloodhound/cmd/api/src/model"
	"github.com/specterops/bloodhound/cmd/api/src/model/appcfg"
	"github.com/specterops/bloodhound/cmd/api/src/test/integration/utils"
	"github.com/specterops/bloodhound/cmd/api/src/test/lab/fixtures"
	"github.com/specterops/bloodhound/packages/go/lab"
	"github.com/stretchr/testify/require"
)

func testCondition(role auth.RoleTemplate, permission model.Permission) string {
	if role.Permissions.Has(permission) {
		return "SHOULD"
	}
	return "SHOULD NOT"
}

func requireForbidden(assert *require.Assertions, err error) {
	var errByte []byte
	errByte, err = json.Marshal(err)
	assert.Nil(err)

	errWrapper := api.ErrorWrapper{}
	err = json.Unmarshal(errByte, &errWrapper)
	assert.Nilf(err, "Failed to unmarshal error %v", string(errByte))
	assert.Equal(errWrapper.HTTPStatus, http.StatusForbidden)
}

func testRoleAccess(t *testing.T, roleName string) {
	role, ok := auth.Roles()[roleName]
	require.Truef(t, ok, "invalid role name")

	customCfg, err := utils.LoadIntegrationTestConfig()
	require.Nil(t, err)

	// In order to role test access, enable_cypher_mutations must be true
	customCfg.EnableCypherMutations = true

	harness := lab.NewHarness()
	customCfgFixture := fixtures.NewCustomConfigFixture(customCfg)
	lab.Pack(harness, customCfgFixture)
	customApiFixture := fixtures.NewCustomApiFixture(customCfgFixture)
	lab.Pack(harness, customApiFixture)
	adminApiClientFixture := fixtures.NewAdminApiClientFixture(customCfgFixture, customApiFixture)
	lab.Pack(harness, adminApiClientFixture)
	userClientFixture := fixtures.NewUserApiClientFixture(fixtures.ConfigFixture, adminApiClientFixture, role.Name)
	lab.Pack(harness, userClientFixture)

	lab.NewSpec(t, harness).Run(
		lab.TestCase(fmt.Sprintf("%s be able to access AppReadApplicationConfiguration endpoints", testCondition(role, auth.Permissions().AppReadApplicationConfiguration)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			_, err := userClient.GetAppConfigs()
			if role.Permissions.Has(auth.Permissions().AppReadApplicationConfiguration) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access AppWriteApplicationConfiguration endpoints", testCondition(role, auth.Permissions().AppWriteApplicationConfiguration)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			updatedPasswordExpirationWindowParameter := appcfg.AppConfigUpdateRequest{
				Key: string(appcfg.PasswordExpirationWindow),
				Value: map[string]any{
					"duration": "P30D",
				},
			}
			_, err := userClient.PutAppConfig(updatedPasswordExpirationWindowParameter)
			if role.Permissions.Has(auth.Permissions().AppWriteApplicationConfiguration) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access own AuthCreateToken endpoints", testCondition(role, auth.Permissions().AuthCreateToken)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			user, err := userClient.GetSelf()
			assert.Nilf(err, "failed looking up user details")

			_, err = userClient.ListUserTokens(user.ID)
			if role.Permissions.Has(auth.Permissions().AuthCreateToken) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s NOT be able to access others AuthCreateToken endpoints unless admin", testCondition(role, auth.Permissions().AuthCreateToken)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			randoUser, err := uuid.NewV4()
			assert.Nilf(err, "failed to create rando user")

			_, err = userClient.ListUserTokens(randoUser)
			if role.Name == auth.RoleAdministrator {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access AuthManageProviders endpoints", testCondition(role, auth.Permissions().AuthManageProviders)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			// TODO when formally deprecated update this to another endpoint
			_, err := userClient.ListSAMLIdentityProviders()
			if role.Permissions.Has(auth.Permissions().AuthManageProviders) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access AuthManageSelf endpoints", testCondition(role, auth.Permissions().AuthManageSelf)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			_, err := userClient.ListPermissions()
			if role.Permissions.Has(auth.Permissions().AuthManageSelf) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access AuthManageUsers endpoints", testCondition(role, auth.Permissions().AuthManageUsers)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			_, err := userClient.ListAuditLogs(time.Now(), time.Now(), 0, 0)
			if role.Permissions.Has(auth.Permissions().AuthManageUsers) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access GraphDBMutate endpoints", testCondition(role, auth.Permissions().GraphDBMutate)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			_, err := userClient.CypherQuery(v2.CypherQueryPayload{Query: "match (w) where w.name = 'voldemort' remove w.name return w"})
			if role.Permissions.Has(auth.Permissions().GraphDBMutate) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access GraphDBWrite endpoints", testCondition(role, auth.Permissions().GraphDBWrite)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			_, err := userClient.CreateAssetGroup(v2.CreateAssetGroupRequest{Name: "test", Tag: "test"})
			if role.Permissions.Has(auth.Permissions().GraphDBWrite) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access GraphDBIngest endpoints", testCondition(role, auth.Permissions().GraphDBIngest)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			_, err := userClient.CreateFileUploadTask()
			if role.Permissions.Has(auth.Permissions().GraphDBIngest) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access GraphDBRead endpoints", testCondition(role, auth.Permissions().GraphDBRead)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			_, err := userClient.ListAssetGroups()
			if role.Permissions.Has(auth.Permissions().GraphDBRead) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access SavedQueriesRead endpoints", testCondition(role, auth.Permissions().SavedQueriesRead)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			_, err := userClient.ListSavedQueries()
			if role.Permissions.Has(auth.Permissions().SavedQueriesRead) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access SavedQueriesWrite endpoints", testCondition(role, auth.Permissions().SavedQueriesWrite)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			_, err := userClient.CreateSavedQuery()
			if role.Permissions.Has(auth.Permissions().SavedQueriesWrite) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),

		lab.TestCase(fmt.Sprintf("%s be able to access WipeDB endpoints", testCondition(role, auth.Permissions().WipeDB)), func(assert *require.Assertions, harness *lab.Harness) {
			userClient, ok := lab.Unpack(harness, userClientFixture)
			assert.True(ok)

			err := userClient.HandleDatabaseWipe(v2.DatabaseWipe{DeleteCollectedGraphData: true})
			if role.Permissions.Has(auth.Permissions().WipeDB) {
				assert.Nil(err)
			} else {
				requireForbidden(assert, err)
			}
		}),
	)
}

func TestRole_ReadOnly(t *testing.T) {
	testRoleAccess(t, auth.RoleReadOnly)
}

func TestRole_UploadOnly(t *testing.T) {
	testRoleAccess(t, auth.RoleUploadOnly)
}

func TestRole_User(t *testing.T) {
	testRoleAccess(t, auth.RoleUser)
}

func TestRole_PowerUser(t *testing.T) {
	testRoleAccess(t, auth.RolePowerUser)
}

func TestRole_Administrator(t *testing.T) {
	testRoleAccess(t, auth.RoleAdministrator)
}
