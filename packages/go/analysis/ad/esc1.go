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

package ad

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/specterops/bloodhound/packages/go/analysis"
	"github.com/specterops/bloodhound/packages/go/analysis/impact"
	"github.com/specterops/bloodhound/packages/go/graphschema/ad"
	"github.com/specterops/dawgs/cardinality"
	"github.com/specterops/dawgs/graph"
	"github.com/specterops/dawgs/ops"
	"github.com/specterops/dawgs/query"
	"github.com/specterops/dawgs/traversal"
	"github.com/specterops/dawgs/util/channels"
)

func PostADCSESC1(ctx context.Context, tx graph.Transaction, outC chan<- analysis.CreatePostRelationshipJob, expandedGroups impact.PathAggregator, enterpriseCA *graph.Node, targetDomains *graph.NodeSet, cache ADCSCache) error {
	results := cardinality.NewBitmap64()
	if publishedCertTemplates := cache.GetPublishedTemplateCache(enterpriseCA.ID); len(publishedCertTemplates) == 0 {
		return nil
	} else {
		ecaEnrollers := cache.GetEnterpriseCAEnrollers(enterpriseCA.ID)
		for _, certTemplate := range publishedCertTemplates {
			if valid, err := isCertTemplateValidForEsc1(certTemplate); err != nil {
				slog.WarnContext(ctx, fmt.Sprintf("Error validating cert template %d: %v", certTemplate.ID, err))
				continue
			} else if !valid {
				continue
			} else {
				results.Or(CalculateCrossProductNodeSets(tx, expandedGroups, cache.GetCertTemplateEnrollers(certTemplate.ID), ecaEnrollers))
			}
		}
	}

	results.Each(func(value uint64) bool {
		for _, domain := range targetDomains.Slice() {
			channels.Submit(ctx, outC, analysis.CreatePostRelationshipJob{
				FromID: graph.ID(value),
				ToID:   domain.ID,
				Kind:   ad.ADCSESC1,
			})
		}
		return true
	})
	return nil
}

func isCertTemplateValidForEsc1(ct *graph.Node) (bool, error) {
	if reqManagerApproval, err := ct.Properties.Get(ad.RequiresManagerApproval.String()).Bool(); err != nil {
		return false, err
	} else if reqManagerApproval {
		return false, nil
	} else if authenticationEnabled, err := ct.Properties.Get(ad.AuthenticationEnabled.String()).Bool(); err != nil {
		return false, err
	} else if !authenticationEnabled {
		return false, nil
	} else if enrolleeSuppliesSubject, err := ct.Properties.Get(ad.EnrolleeSuppliesSubject.String()).Bool(); err != nil {
		return false, err
	} else if !enrolleeSuppliesSubject {
		return false, nil
	} else if schemaVersion, err := ct.Properties.Get(ad.SchemaVersion.String()).Float64(); err != nil {
		return false, err
	} else if authorizedSignatures, err := ct.Properties.Get(ad.AuthorizedSignatures.String()).Float64(); err != nil {
		return false, err
	} else if schemaVersion > 1 && authorizedSignatures > 0 {
		return false, nil
	} else {
		return true, nil
	}
}

func ADCSESC1Path1Pattern(domainID graph.ID) traversal.PatternContinuation {
	return traversal.NewPattern().OutboundWithDepth(0, 0, query.And(
		query.Kind(query.Relationship(), ad.MemberOf),
		query.Kind(query.End(), ad.Group),
	)).
		Outbound(query.And(
			query.KindIn(query.Relationship(), ad.GenericAll, ad.Enroll, ad.AllExtendedRights),
			query.Kind(query.End(), ad.CertTemplate),
			query.Or(
				query.And(
					query.Equals(query.EndProperty(ad.RequiresManagerApproval.String()), false),
					query.GreaterThan(query.EndProperty(ad.SchemaVersion.String()), 1),
					query.Equals(query.EndProperty(ad.AuthorizedSignatures.String()), 0),
					query.Equals(query.EndProperty(ad.AuthenticationEnabled.String()), true),
					query.Equals(query.EndProperty(ad.EnrolleeSuppliesSubject.String()), true),
				),
				query.And(
					query.Equals(query.EndProperty(ad.RequiresManagerApproval.String()), false),
					query.Equals(query.EndProperty(ad.SchemaVersion.String()), 1),
					query.Equals(query.EndProperty(ad.AuthenticationEnabled.String()), true),
					query.Equals(query.EndProperty(ad.EnrolleeSuppliesSubject.String()), true),
				),
			),
		)).
		Outbound(query.And(
			query.KindIn(query.Relationship(), ad.PublishedTo),
			query.Kind(query.End(), ad.EnterpriseCA),
		)).
		OutboundWithDepth(0, 0, query.And(
			query.KindIn(query.Relationship(), ad.IssuedSignedBy, ad.EnterpriseCAFor),
			query.KindIn(query.End(), ad.EnterpriseCA, ad.AIACA),
		)).
		Outbound(query.And(
			query.KindIn(query.Relationship(), ad.IssuedSignedBy, ad.EnterpriseCAFor),
			query.Kind(query.End(), ad.RootCA),
		)).
		Outbound(query.And(
			query.KindIn(query.Relationship(), ad.RootCAFor),
			query.Equals(query.EndID(), domainID),
		))
}

func ADCSESC1Path2Pattern(domainID graph.ID, enterpriseCAs cardinality.Duplex[uint64]) traversal.PatternContinuation {
	return traversal.NewPattern().OutboundWithDepth(0, 0, query.And(
		query.Kind(query.Relationship(), ad.MemberOf),
		query.Kind(query.End(), ad.Group),
	)).
		Outbound(query.And(
			query.KindIn(query.Relationship(), ad.Enroll),
			query.InIDs(query.EndID(), graph.DuplexToGraphIDs(enterpriseCAs)...),
		)).
		Outbound(query.And(
			query.KindIn(query.Relationship(), ad.TrustedForNTAuth),
			query.Kind(query.End(), ad.NTAuthStore),
		)).
		Outbound(query.And(
			query.KindIn(query.Relationship(), ad.NTAuthStoreFor),
			query.Equals(query.EndID(), domainID),
		))
}

func GetADCSESC1EdgeComposition(ctx context.Context, db graph.Database, edge *graph.Relationship) (graph.PathSet, error) {
	/*
		MATCH (u:User {objectid:'S-1-5-21-2057499049-1289676208-1959431660-238209'})-[:ADCSESC1]->(d:Domain {objectid:'S-1-5-21-1621856376-872934182-3936853371'})
		MATCH (c:Container)-[:Contains]->(rca:RootCA)
		WHERE c.name CONTAINS "CERTIFICATION AUTHORITIES" AND c.domain = d.domain
		MATCH (ca:EnterpriseCA)-[:IssuedSignedBy|EnterpriseCAFor*1..]->(rca)
		WHERE (ca)-[:TrustedForNTAuth]->(:NTAuthStore)
		MATCH (ct:CertTemplate)-[:PublishedTo]->(ca)
		WHERE (ct.requiresmanagerapproval = false
		AND ct.schemaversion > 1
		AND ct.authorizedsignatures = 0
		AND ct.authenticationenabled = true
		AND ct.enrolleesuppliessubject = true)
		OR (ct.requiresmanagerapproval = false
		AND ct.schemaversion = 1
		AND ct.authenticationenabled = true
		AND ct.enrolleesuppliessubject = true)
		OPTIONAL MATCH p1 = (u)-[:GenericAll|Enroll|AllExtendedRights]->(ct)-[:PublishedTo]->(ca)
		OPTIONAL MATCH p2 = (u)-[:MemberOf*1..]->(:Group)-[:GenericAll|Enroll|AllExtendedRights]->(ct)-[:PublishedTo]->(ca)
		OPTIONAL MATCH p3 = (u)-[:Enroll]->(ca)-[:IssuedSignedBy|EnterpriseCAFor|RootCAFor*1..]->(d)
		OPTIONAL MATCH p4 = (u)-[:MemberOf*1..]->(:Group)-[:Enroll]->(ca)-[:IssuedSignedBy|EnterpriseCAFor|RootCAFor*1..]->(d)
		OPTIONAL MATCH p5 = (ca)-[:TrustedForNTAuth]->(:NTAuthStore)-[:NTAuthStoreFor]->(d)
		RETURN p1,p2,p3,p4,p5
	*/
	var (
		startNode  *graph.Node
		startNodes = graph.NodeSet{}

		traversalInst      = traversal.New(db, analysis.MaximumDatabaseParallelWorkers)
		paths              = graph.PathSet{}
		candidateSegments  = map[graph.ID][]*graph.PathSegment{}
		path1EnterpriseCAs = cardinality.NewBitmap64()
		path2EnterpriseCAs = cardinality.NewBitmap64()
		lock               = &sync.Mutex{}
	)

	if err := db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		var err error
		if startNode, err = ops.FetchNode(tx, edge.StartID); err != nil {
			return err
		} else {
			return nil
		}
	}); err != nil {
		return nil, err
	}

	// Add startnode, Auth. Users, and Everyone to start nodes
	if err := db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		if nodeSet, err := FetchAuthUsersAndEveryoneGroups(tx); err != nil {
			return err
		} else {
			startNodes.AddSet(nodeSet)
			return nil
		}
	}); err != nil {
		return nil, err
	}
	startNodes.Add(startNode)

	// P1
	for _, n := range startNodes.Slice() {
		if err := traversalInst.BreadthFirst(ctx, traversal.Plan{
			Root: n,
			Driver: ADCSESC1Path1Pattern(edge.EndID).Do(func(terminal *graph.PathSegment) error {
				// Find the first enterprise CA and track it before stuffing this path into the candidates
				var enterpriseCANode *graph.Node
				terminal.WalkReverse(func(nextSegment *graph.PathSegment) bool {
					if nextSegment.Node.Kinds.ContainsOneOf(ad.EnterpriseCA) {
						enterpriseCANode = nextSegment.Node
					}
					return true
				})

				lock.Lock()
				candidateSegments[enterpriseCANode.ID] = append(candidateSegments[enterpriseCANode.ID], terminal)
				path1EnterpriseCAs.Add(enterpriseCANode.ID.Uint64())
				lock.Unlock()

				return nil
			}),
		}); err != nil {
			return nil, err
		}
	}

	// P2
	for _, n := range startNodes.Slice() {
		if err := traversalInst.BreadthFirst(ctx, traversal.Plan{
			Root: n,
			Driver: ADCSESC1Path2Pattern(edge.EndID, path1EnterpriseCAs).Do(func(terminal *graph.PathSegment) error {
				// Find the CA and track it before stuffing this path into the candidates
				enterpriseCANode := terminal.Search(func(nextSegment *graph.PathSegment) bool {
					return nextSegment.Node.Kinds.ContainsOneOf(ad.EnterpriseCA)
				})

				lock.Lock()
				candidateSegments[enterpriseCANode.ID] = append(candidateSegments[enterpriseCANode.ID], terminal)
				path2EnterpriseCAs.Add(enterpriseCANode.ID.Uint64())
				lock.Unlock()

				return nil
			}),
		}); err != nil {
			return nil, err
		}
	}

	// Intersect the CAs and take only those seen in both paths
	path1EnterpriseCAs.And(path2EnterpriseCAs)

	// Render paths from the segments
	path1EnterpriseCAs.Each(func(value uint64) bool {
		for _, segment := range candidateSegments[graph.ID(value)] {
			paths.AddPath(segment.Path())
		}

		return true
	})

	return paths, nil
}

func getGoldenCertEdgeComposition(tx graph.Transaction, edge *graph.Relationship) (graph.PathSet, error) {
	finalPaths := graph.NewPathSet()
	//Grab the start node (computer) as well as the target domain node
	if startNode, targetDomainNode, err := ops.FetchRelationshipNodes(tx, edge); err != nil {
		return finalPaths, err
	} else {
		//Find hosted enterprise CA
		if ecaPaths, err := ops.FetchPathSet(tx.Relationships().Filter(query.And(
			query.Equals(query.StartID(), startNode.ID),
			query.KindIn(query.End(), ad.EnterpriseCA),
			query.KindIn(query.Relationship(), ad.HostsCAService),
		))); err != nil {
			slog.Error(fmt.Sprintf("Error getting hostscaservice edge to enterprise ca for computer %d : %v", startNode.ID, err))
		} else {
			for _, ecaPath := range ecaPaths {
				eca := ecaPath.Terminal()
				if chainToRootCAPaths, err := FetchEnterpriseCAsCertChainPathToDomain(tx, eca, targetDomainNode); err != nil {
					slog.Error(fmt.Sprintf("Error getting eca %d path to domain %d: %v", eca.ID, targetDomainNode.ID, err))
				} else if chainToRootCAPaths.Len() == 0 {
					continue
				} else if trustedForAuthPaths, err := FetchEnterpriseCAsTrustedForAuthPathToDomain(tx, eca, targetDomainNode); err != nil {
					slog.Error(fmt.Sprintf("Error getting eca %d path to domain %d via trusted for auth: %v", eca.ID, targetDomainNode.ID, err))
				} else if trustedForAuthPaths.Len() == 0 {
					continue
				} else {
					finalPaths.AddPath(ecaPath)
					finalPaths.AddPathSet(chainToRootCAPaths)
					finalPaths.AddPathSet(trustedForAuthPaths)
				}
			}
		}

		return finalPaths, nil
	}
}
