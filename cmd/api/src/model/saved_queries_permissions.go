//
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

package model

import (
	"github.com/gofrs/uuid"
)

type SavedQueryScope string

const (
	SavedQueryScopeOwned  SavedQueryScope = "owned"
	SavedQueryScopeShared SavedQueryScope = "shared"
	SavedQueryScopePublic SavedQueryScope = "public"
	SavedQueryScopeAll    SavedQueryScope = "all"
)

// SavedQueriesPermissions represents the database model which allows users to share saved cypher queries
type SavedQueriesPermissions struct {
	SharedToUserID uuid.NullUUID `json:"shared_to_user_id"`
	QueryID        int64         `json:"query_id"`
	Public         bool          `json:"public"`

	BigSerial
}
