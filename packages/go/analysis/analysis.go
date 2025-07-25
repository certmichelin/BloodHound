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

package analysis

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/specterops/bloodhound/packages/go/graphschema"
	"github.com/specterops/bloodhound/packages/go/graphschema/ad"
	"github.com/specterops/bloodhound/packages/go/graphschema/azure"
	"github.com/specterops/bloodhound/packages/go/graphschema/common"
	"github.com/specterops/bloodhound/packages/go/slicesext"
	"github.com/specterops/dawgs/graph"
	"github.com/specterops/dawgs/ops"
	"github.com/specterops/dawgs/query"
)

const (
	NodeKindUnknown                = "Unknown"
	MaximumDatabaseParallelWorkers = 6
)

type CompositionCounter struct {
	counter atomic.Int64
}

func (c *CompositionCounter) Get() int64 {
	ret := c.counter.Load()
	c.counter.Add(1)
	return ret
}

func NewCompositionCounter() CompositionCounter {
	return CompositionCounter{
		counter: atomic.Int64{},
	}
}

func GetNodeKindDisplayLabel(node *graph.Node) string {
	return GetNodeKind(node).String()
}

func GetNodeKind(node *graph.Node) graph.Kind {
	return graphschema.PrimaryNodeKind(node.Kinds)
}

func ParseKind(rawKind string) (graph.Kind, error) {
	for kind := range graphschema.ValidKinds {
		if kind.String() == rawKind {
			return kind, nil
		}
	}

	return nil, fmt.Errorf("unknown kind %s", rawKind)
}

func ParseKinds(rawKinds ...string) (graph.Kinds, error) {
	if len(rawKinds) == 0 {
		return graph.Kinds{ad.Entity, azure.Entity}, nil
	}

	return slicesext.MapWithErr(rawKinds, ParseKind)
}

func nodeByIndexedKindProperty(property, value string, kind graph.Kind) graph.Criteria {
	return query.And(
		query.Equals(query.NodeProperty(property), value),
		query.Kind(query.Node(), kind),
	)
}

// FetchNodeByObjectID will search for a node given its object ID. This function may run more than one query to ensure
// that label indexes are correctly exercised. The turnaround time of multiple indexed queries is an order of magnitude
// faster in larger environments than allowing Neo4j to perform a table scan of unindexed node properties.
func FetchNodeByObjectID(tx graph.Transaction, objectID string) (*graph.Node, error) {
	if node, err := tx.Nodes().Filter(nodeByIndexedKindProperty(common.ObjectID.String(), objectID, ad.Entity)).First(); err != nil {
		if !graph.IsErrNotFound(err) {
			return nil, err
		}
	} else {
		return node, nil
	}

	return tx.Nodes().Filter(nodeByIndexedKindProperty(common.ObjectID.String(), objectID, azure.Entity)).First()
}

func FetchEdgeByStartAndEnd(ctx context.Context, graphDB graph.Database, start, end graph.ID, edgeKind graph.Kind) (*graph.Relationship, error) {
	var result *graph.Relationship
	return result, graphDB.ReadTransaction(ctx, func(tx graph.Transaction) error {
		if rel, err := tx.Relationships().Filter(query.And(
			query.Equals(query.StartID(), start),
			query.Equals(query.EndID(), end),
			query.Kind(query.Relationship(), edgeKind),
		)).First(); err != nil {
			return err
		} else {
			result = rel
			return nil
		}
	})
}

func ExpandGroupMembershipPaths(tx graph.Transaction, candidates graph.NodeSet) (graph.PathSet, error) {
	groupMemberPaths := graph.NewPathSet()

	for _, candidate := range candidates {
		if candidate.Kinds.ContainsOneOf(ad.Group) {
			if membershipPaths, err := ops.TraversePaths(tx, ops.TraversalPlan{
				Root:      candidate,
				Direction: graph.DirectionInbound,
				BranchQuery: func() graph.Criteria {
					return query.Kind(query.Relationship(), ad.MemberOf)
				},
			}); err != nil {
				return nil, err
			} else {
				groupMemberPaths.AddPathSet(membershipPaths)
			}
		}
	}

	return groupMemberPaths, nil
}

func FromEntityToEntityWithRelationshipKind(tx graph.Transaction, target *graph.Node, relKind graph.Kind) graph.RelationshipQuery {
	return tx.Relationships().Filterf(func() graph.Criteria {
		filters := []graph.Criteria{
			query.Kind(query.Start(), ad.Entity),
			query.Kind(query.Relationship(), relKind),
			query.Equals(query.EndID(), target.ID),
		}

		return query.And(filters...)
	})
}

type PathDelegate = func(tx graph.Transaction, node *graph.Node) (graph.PathSet, error)
type ListDelegate = func(tx graph.Transaction, node *graph.Node, skip, limit int) (graph.NodeSet, error)

type ParallelPathDelegate = func(ctx context.Context, db graph.Database, node *graph.Node) (graph.PathSet, error)
type ParallelListDelegate = func(ctx context.Context, db graph.Database, node *graph.Node, skip int, limit int) (graph.NodeSet, error)
