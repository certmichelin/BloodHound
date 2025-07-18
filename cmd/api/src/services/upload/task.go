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

package upload

import (
	"context"

	"github.com/specterops/bloodhound/cmd/api/src/database/types/null"
	"github.com/specterops/bloodhound/cmd/api/src/model"
)

type IngestTaskParams struct {
	Filename  string
	FileType  model.FileType
	RequestID string
	JobID     int64
}

func CreateIngestTask(ctx context.Context, db UploadData, params IngestTaskParams) (model.IngestTask, error) {
	newIngestTask := model.IngestTask{
		FileName:    params.Filename,
		RequestGUID: params.RequestID,
		JobId:       null.Int64From(params.JobID),
		FileType:    params.FileType,
	}

	return db.CreateIngestTask(ctx, newIngestTask)
}

func CreateCompositionInfo(ctx context.Context, db UploadData, nodes model.EdgeCompositionNodes, edges model.EdgeCompositionEdges) (model.EdgeCompositionNodes, model.EdgeCompositionEdges, error) {
	return db.CreateCompositionInfo(ctx, nodes, edges)
}
