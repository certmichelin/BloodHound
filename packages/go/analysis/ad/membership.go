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

package ad

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/specterops/bloodhound/packages/go/analysis"
	"github.com/specterops/bloodhound/packages/go/analysis/impact"
	"github.com/specterops/bloodhound/packages/go/bhlog/measure"
	"github.com/specterops/bloodhound/packages/go/graphschema/ad"
	"github.com/specterops/dawgs/cardinality"
	"github.com/specterops/dawgs/graph"
	"github.com/specterops/dawgs/ops"
	"github.com/specterops/dawgs/query"
	"github.com/specterops/dawgs/traversal"
)

func ResolveAllGroupMemberships(ctx context.Context, db graph.Database, additionalCriteria ...graph.Criteria) (impact.PathAggregator, error) {
	defer measure.ContextMeasure(ctx, slog.LevelInfo, "ResolveAllGroupMemberships")()

	var (
		adGroupIDs []graph.ID

		searchCriteria = []graph.Criteria{query.KindIn(query.Relationship(), ad.MemberOf, ad.MemberOfLocalGroup)}
		traversalMap   = cardinality.ThreadSafeDuplex(cardinality.NewBitmap64())
		traversalInst  = traversal.NewIDTraversal(db, analysis.MaximumDatabaseParallelWorkers)
		memberships    = impact.NewThreadSafeAggregator(impact.NewIDA(func() cardinality.Provider[uint64] {
			return cardinality.NewBitmap64()
		}))
	)

	if len(additionalCriteria) > 0 {
		searchCriteria = append(searchCriteria, additionalCriteria...)
	}

	if err := db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		if fetchedGroups, err := ops.FetchNodeIDs(tx.Nodes().Filter(
			query.KindIn(query.Node(), ad.Group, ad.LocalGroup),
		)); err != nil {
			return err
		} else {
			adGroupIDs = fetchedGroups
			return nil
		}
	}); err != nil {
		return memberships, err
	}

	slog.InfoContext(ctx, fmt.Sprintf("Collected %d groups to resolve", len(adGroupIDs)))

	for _, adGroupID := range adGroupIDs {
		if traversalMap.Contains(adGroupID.Uint64()) {
			continue
		}

		if err := traversalInst.BreadthFirst(ctx, traversal.IDPlan{
			Root: adGroupID,
			Delegate: func(ctx context.Context, tx graph.Transaction, segment *graph.IDSegment) ([]*graph.IDSegment, error) {
				if nextQuery, err := newTraversalQuery(tx, segment, graph.DirectionInbound, searchCriteria...); err != nil {
					return nil, err
				} else {
					var nextSegments []*graph.IDSegment

					if err := nextQuery.FetchTriples(
						func(cursor graph.Cursor[graph.RelationshipTripleResult]) error {
							for nextTriple := range cursor.Chan() {
								if traversalMap.CheckedAdd(nextTriple.StartID.Uint64()) {
									nextSegments = append(nextSegments, segment.Descend(nextTriple.StartID, nextTriple.ID))
								} else {
									memberships.AddShortcut(segment.Descend(nextTriple.StartID, nextTriple.ID))
								}
							}

							return cursor.Error()
						}); err != nil {
						return nil, err
					}

					// Is this path terminal?
					if len(nextSegments) == 0 {
						memberships.AddPath(segment)
					}

					return nextSegments, nil
				}
			},
		}); err != nil {
			return nil, err
		}
	}

	return memberships, nil
}

func newTraversalQuery(tx graph.Transaction, segment *graph.IDSegment, direction graph.Direction, queryCriteria ...graph.Criteria) (graph.RelationshipQuery, error) {
	var (
		traversalCriteria []graph.Criteria
	)

	switch direction {
	case graph.DirectionInbound:
		traversalCriteria = append(traversalCriteria,
			query.Equals(query.EndID(), query.Parameter(segment.Node)),
			query.Not(
				query.Equals(query.StartID(), query.Parameter(segment.Node)),
			),
		)

	case graph.DirectionOutbound:
		traversalCriteria = append(traversalCriteria,
			query.Equals(query.StartID(), query.Parameter(segment.Node)),
			query.Not(
				query.Equals(query.EndID(), query.Parameter(segment.Node)),
			),
		)

	default:
		return nil, fmt.Errorf("unsupported direction: %v", direction)
	}

	if len(queryCriteria) > 0 {
		traversalCriteria = append(traversalCriteria, queryCriteria...)
	}

	return tx.Relationships().Filter(query.And(traversalCriteria...)), nil
}

func NodeDuplexByKinds(ctx context.Context, db graph.Database, nodes cardinality.Duplex[uint64]) (*graph.ThreadSafeKindBitmap, error) {
	nodesByKind := graph.NewThreadSafeKindBitmap()

	return nodesByKind, db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		return tx.Nodes().Filter(
			query.InIDs(query.NodeID(), graph.Uint64SliceToIDs(nodes.Slice())...),
		).FetchKinds(func(cursor graph.Cursor[graph.KindsResult]) error {
			for nextResult := range cursor.Chan() {
				for _, kind := range nextResult.Kinds {
					nodesByKind.Add(kind, nextResult.ID.Uint64())
				}
			}

			return cursor.Error()
		})
	})
}

func FetchPathMembers(ctx context.Context, db graph.Database, root graph.ID, direction graph.Direction, queryCriteria ...graph.Criteria) (cardinality.Duplex[uint64], error) {
	traversalMap := cardinality.ThreadSafeDuplex(cardinality.NewBitmap64())

	return traversalMap, traversal.NewIDTraversal(db, analysis.MaximumDatabaseParallelWorkers).BreadthFirst(ctx, traversal.IDPlan{
		Root: root,
		Delegate: func(ctx context.Context, tx graph.Transaction, segment *graph.IDSegment) ([]*graph.IDSegment, error) {
			if nextQuery, err := newTraversalQuery(tx, segment, direction, queryCriteria...); err != nil {
				return nil, err
			} else {
				var nextSegments []*graph.IDSegment

				return nextSegments, nextQuery.FetchTriples(func(cursor graph.Cursor[graph.RelationshipTripleResult]) error {
					for nextTriple := range cursor.Chan() {
						if nextID, err := direction.PickReverseID(nextTriple.StartID, nextTriple.EndID); err != nil {
							return err
						} else if traversalMap.CheckedAdd(nextID.Uint64()) {
							nextSegments = append(nextSegments, segment.Descend(nextID, nextTriple.ID))
						}
					}

					return cursor.Error()
				})
			}
		},
	})
}
