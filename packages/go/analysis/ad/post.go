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
	"errors"
	"fmt"
	"log/slog"

	"github.com/RoaringBitmap/roaring/v2/roaring64"
	"github.com/specterops/bloodhound/packages/go/analysis"
	"github.com/specterops/bloodhound/packages/go/analysis/ad/wellknown"
	"github.com/specterops/bloodhound/packages/go/analysis/impact"
	"github.com/specterops/bloodhound/packages/go/graphschema/ad"
	"github.com/specterops/bloodhound/packages/go/graphschema/common"
	"github.com/specterops/dawgs/cardinality"
	"github.com/specterops/dawgs/graph"
	"github.com/specterops/dawgs/ops"
	"github.com/specterops/dawgs/query"
	"github.com/specterops/dawgs/util/channels"
)

func PostProcessedRelationships() []graph.Kind {
	return []graph.Kind{
		ad.DCSync,
		ad.SyncLAPSPassword,
		ad.CanRDP,
		ad.AdminTo,
		ad.CanPSRemote,
		ad.ExecuteDCOM,
		ad.TrustedForNTAuth,
		ad.IssuedSignedBy,
		ad.EnterpriseCAFor,
		ad.GoldenCert,
		ad.ADCSESC1,
		ad.ADCSESC3,
		ad.ADCSESC4,
		ad.ADCSESC6a,
		ad.ADCSESC6b,
		ad.ADCSESC10a,
		ad.ADCSESC10b,
		ad.ADCSESC9a,
		ad.ADCSESC9b,
		ad.ADCSESC13,
		ad.EnrollOnBehalfOf,
		ad.SyncedToEntraUser,
		ad.Owns,
		ad.WriteOwner,
		ad.ExtendedByPolicy,
		ad.Owns,
		ad.WriteOwner,
		ad.CoerceAndRelayNTLMToADCS,
		ad.CoerceAndRelayNTLMToSMB,
		ad.CoerceAndRelayNTLMToLDAP,
		ad.CoerceAndRelayNTLMToLDAPS,
		ad.GPOAppliesTo,
		ad.CanApplyGPO,
		ad.HasTrustKeys,
	}
}

func PostSyncLAPSPassword(ctx context.Context, db graph.Database, groupExpansions impact.PathAggregator) (*analysis.AtomicPostProcessingStats, error) {
	if domainNodes, err := fetchCollectedDomainNodes(ctx, db); err != nil {
		return &analysis.AtomicPostProcessingStats{}, err
	} else {
		operation := analysis.NewPostRelationshipOperation(ctx, db, "SyncLAPSPassword Post Processing")
		for _, domain := range domainNodes {
			innerDomain := domain
			operation.Operation.SubmitReader(func(ctx context.Context, tx graph.Transaction, outC chan<- analysis.CreatePostRelationshipJob) error {
				if lapsSyncers, err := getLAPSSyncers(tx, innerDomain, groupExpansions); err != nil {
					return err
				} else if lapsSyncers.Cardinality() == 0 {
					return nil
				} else if computers, err := getLAPSComputersForDomain(tx, innerDomain); err != nil {
					return err
				} else {
					for _, computer := range computers {
						lapsSyncers.Each(func(value uint64) bool {
							channels.Submit(ctx, outC, analysis.CreatePostRelationshipJob{
								FromID: graph.ID(value),
								ToID:   computer,
								Kind:   ad.SyncLAPSPassword,
							})
							return true
						})
					}

					return nil
				}
			})
		}

		return &operation.Stats, operation.Done()
	}
}

func PostDCSync(ctx context.Context, db graph.Database, groupExpansions impact.PathAggregator) (*analysis.AtomicPostProcessingStats, error) {
	if domainNodes, err := fetchCollectedDomainNodes(ctx, db); err != nil {
		return &analysis.AtomicPostProcessingStats{}, err
	} else {
		operation := analysis.NewPostRelationshipOperation(ctx, db, "DCSync Post Processing")

		for _, domain := range domainNodes {
			innerDomain := domain
			operation.Operation.SubmitReader(func(ctx context.Context, tx graph.Transaction, outC chan<- analysis.CreatePostRelationshipJob) error {
				if dcSyncers, err := getDCSyncers(tx, innerDomain, groupExpansions); err != nil {
					return err
				} else if dcSyncers.Cardinality() == 0 {
					return nil
				} else {
					dcSyncers.Each(func(value uint64) bool {
						channels.Submit(ctx, outC, analysis.CreatePostRelationshipJob{
							FromID: graph.ID(value),
							ToID:   innerDomain.ID,
							Kind:   ad.DCSync,
						})
						return true
					})

					return nil
				}
			})
		}

		return &operation.Stats, operation.Done()
	}
}

func PostHasTrustKeys(ctx context.Context, db graph.Database) (*analysis.AtomicPostProcessingStats, error) {
	if domainNodes, err := fetchCollectedDomainNodes(ctx, db); err != nil {
		return &analysis.AtomicPostProcessingStats{}, err
	} else {
		operation := analysis.NewPostRelationshipOperation(ctx, db, "HasTrustKeys Post Processing")
		if err := operation.Operation.SubmitReader(func(ctx context.Context, tx graph.Transaction, outC chan<- analysis.CreatePostRelationshipJob) error {
			for _, domain := range domainNodes {
				if netbios, err := domain.Properties.Get(ad.NetBIOS.String()).String(); err != nil {
					// The property is new and may therefore not exist
					slog.DebugContext(ctx, fmt.Sprintf("Skipping domain %d: missing NetBIOS property", domain.ID))
					continue
				} else if trustingDomains, err := getDirectOutboundTrustDomains(tx, domain); err != nil {
					slog.ErrorContext(ctx, fmt.Sprintf("Error getting outbound trust edges from domain %d: %v", domain.ID, err))
					continue
				} else {
					for _, trustingDomain := range trustingDomains {
						if trustingDomainSid, err := trustingDomain.Properties.Get(ad.DomainSID.String()).String(); err != nil {
							// DomainSID is only created after we have performed collection of the domain
							slog.DebugContext(ctx, fmt.Sprintf("Skipping trusting domain %d: missing DomainSID property", trustingDomain.ID))
							continue
						} else if trustAccount, err := getTrustAccount(tx, trustingDomainSid, netbios); err != nil {
							// The account may not exist if we have not collected it
							slog.DebugContext(ctx, fmt.Sprintf("Trust account not found for domain SID %s and NetBIOS %s", trustingDomainSid, netbios))
							continue
						} else {
							channels.Submit(ctx, outC, analysis.CreatePostRelationshipJob{
								FromID: domain.ID,
								ToID:   trustAccount.ID,
								Kind:   ad.HasTrustKeys,
							})
						}
					}
				}
			}
			return nil
		}); err != nil {
			return &analysis.AtomicPostProcessingStats{}, fmt.Errorf("error creating HasTrustKeys edges: %w", err)
		}

		return &operation.Stats, operation.Done()
	}
}

func FetchComputers(ctx context.Context, db graph.Database) (*roaring64.Bitmap, error) {
	computerNodeIds := roaring64.NewBitmap()

	return computerNodeIds, db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		return tx.Nodes().Filterf(func() graph.Criteria {
			return query.Kind(query.Node(), ad.Computer)
		}).FetchIDs(func(cursor graph.Cursor[graph.ID]) error {
			for id := range cursor.Chan() {
				computerNodeIds.Add(id.Uint64())
			}

			return nil
		})
	})
}

func FetchNodesByKind(ctx context.Context, db graph.Database, kinds ...graph.Kind) ([]*graph.Node, error) {
	var nodes []*graph.Node
	return nodes, db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		var err error
		if nodes, err = ops.FetchNodes(tx.Nodes().Filterf(func() graph.Criteria {
			return query.And(
				query.KindIn(query.Node(), kinds...),
			)
		})); err != nil {
			return err
		} else {
			return nil
		}
	})
}

func fetchCollectedDomainNodes(ctx context.Context, db graph.Database) ([]*graph.Node, error) {
	var nodes []*graph.Node
	return nodes, db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		var err error
		if nodes, err = ops.FetchNodes(tx.Nodes().Filterf(func() graph.Criteria {
			return query.And(
				query.Kind(query.Node(), ad.Domain),
				query.Equals(query.NodeProperty(common.Collected.String()), true),
			)
		})); err != nil {
			return err
		} else {
			return nil
		}
	})
}

func getDirectOutboundTrustDomains(tx graph.Transaction, domain *graph.Node) (graph.NodeSet, error) {
	return ops.FetchEndNodes(tx.Relationships().Filterf(func() graph.Criteria {
		return query.And(
			query.Equals(query.StartID(), domain.ID),
			query.KindIn(query.Relationship(), ad.SameForestTrust, ad.CrossForestTrust),
			query.Kind(query.End(), ad.Domain),
		)
	}))
}

func getTrustAccount(tx graph.Transaction, domainSid, netbios string) (*graph.Node, error) {
	nodes, err := ops.FetchNodes(tx.Nodes().Filterf(func() graph.Criteria {
		return query.And(
			query.Kind(query.Node(), ad.User),
			query.Equals(query.NodeProperty(ad.DomainSID.String()), domainSid),
			query.Equals(query.NodeProperty(ad.SamAccountName.String()), netbios+"$"),
		)
	}).Limit(1))
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, graph.ErrNoResultsFound
	}
	return nodes[0], err
}
func getLAPSSyncers(tx graph.Transaction, domain *graph.Node, groupExpansions impact.PathAggregator) (cardinality.Duplex[uint64], error) {
	var (
		getChangesQuery         = analysis.FromEntityToEntityWithRelationshipKind(tx, domain, ad.GetChanges)
		getChangesFilteredQuery = analysis.FromEntityToEntityWithRelationshipKind(tx, domain, ad.GetChangesInFilteredSet)
	)

	if getChangesNodes, err := ops.FetchStartNodes(getChangesQuery); err != nil {
		return nil, err
	} else if getChangesFilteredNodes, err := ops.FetchStartNodes(getChangesFilteredQuery); err != nil {
		return nil, err
	} else {
		results := CalculateCrossProductNodeSets(tx, groupExpansions, getChangesNodes.Slice(), getChangesFilteredNodes.Slice())

		return results, nil
	}
}

func getDCSyncers(tx graph.Transaction, domain *graph.Node, groupExpansions impact.PathAggregator) (cardinality.Duplex[uint64], error) {
	var (
		getChangesQuery    = analysis.FromEntityToEntityWithRelationshipKind(tx, domain, ad.GetChanges)
		getChangesAllQuery = analysis.FromEntityToEntityWithRelationshipKind(tx, domain, ad.GetChangesAll)
	)

	if getChangesNodes, err := ops.FetchStartNodes(getChangesQuery); err != nil {
		return nil, err
	} else if getChangesAllNodes, err := ops.FetchStartNodes(getChangesAllQuery); err != nil {
		return nil, err
	} else {
		results := CalculateCrossProductNodeSets(tx, groupExpansions, getChangesNodes.Slice(), getChangesAllNodes.Slice())

		return results, nil
	}
}

func getLAPSComputersForDomain(tx graph.Transaction, domain *graph.Node) ([]graph.ID, error) {
	if domainSid, err := domain.Properties.Get(ad.DomainSID.String()).String(); err != nil {
		return nil, err
	} else {
		return ops.FetchNodeIDs(tx.Nodes().Filterf(func() graph.Criteria {
			return query.And(
				query.Kind(query.Node(), ad.Computer),
				query.Equals(
					query.Property(query.Node(), ad.HasLAPS.String()), true),
				query.Equals(query.Property(query.Node(), ad.DomainSID.String()), domainSid),
			)
		}))
	}
}

func PostLocalGroups(ctx context.Context, db graph.Database, localGroupExpansions impact.PathAggregator, enforceURA bool, citrixEnabled bool) (*analysis.AtomicPostProcessingStats, error) {
	var (
		adminGroupSuffix    = "-544"
		psRemoteGroupSuffix = "-580"
		dcomGroupSuffix     = "-562"
	)

	if computers, err := FetchComputers(ctx, db); err != nil {
		return &analysis.AtomicPostProcessingStats{}, err
	} else {
		var (
			threadSafeLocalGroupExpansions = impact.NewThreadSafeAggregator(localGroupExpansions)
			operation                      = analysis.NewPostRelationshipOperation(ctx, db, "LocalGroup Post Processing")
		)

		for idx, computer := range computers.ToArray() {
			computerID := graph.ID(computer)

			if idx > 0 && idx%10000 == 0 {
				slog.InfoContext(ctx, fmt.Sprintf("Post processed %d active directory computers", idx))
			}

			if err := operation.Operation.SubmitReader(func(ctx context.Context, tx graph.Transaction, outC chan<- analysis.CreatePostRelationshipJob) error {
				if entities, err := FetchLocalGroupBitmapForComputer(tx, computerID, dcomGroupSuffix); err != nil {
					return err
				} else {
					for _, admin := range entities.Slice() {
						nextJob := analysis.CreatePostRelationshipJob{
							FromID: graph.ID(admin),
							ToID:   computerID,
							Kind:   ad.ExecuteDCOM,
						}

						if !channels.Submit(ctx, outC, nextJob) {
							return nil
						}
					}

					return nil
				}
			}); err != nil {
				return &analysis.AtomicPostProcessingStats{}, fmt.Errorf("failed submitting reader for operation involving computer %d: %w", computerID, err)
			}

			if err := operation.Operation.SubmitReader(func(ctx context.Context, tx graph.Transaction, outC chan<- analysis.CreatePostRelationshipJob) error {
				if entities, err := FetchLocalGroupBitmapForComputer(tx, computerID, psRemoteGroupSuffix); err != nil {
					return err
				} else {
					for _, admin := range entities.Slice() {
						nextJob := analysis.CreatePostRelationshipJob{
							FromID: graph.ID(admin),
							ToID:   computerID,
							Kind:   ad.CanPSRemote,
						}

						if !channels.Submit(ctx, outC, nextJob) {
							return nil
						}
					}

					return nil
				}
			}); err != nil {
				return &analysis.AtomicPostProcessingStats{}, fmt.Errorf("failed submitting reader for operation involving computer %d: %w", computerID, err)
			}

			if err := operation.Operation.SubmitReader(func(ctx context.Context, tx graph.Transaction, outC chan<- analysis.CreatePostRelationshipJob) error {
				if entities, err := FetchLocalGroupBitmapForComputer(tx, computerID, adminGroupSuffix); err != nil {
					return err
				} else {
					for _, admin := range entities.Slice() {
						nextJob := analysis.CreatePostRelationshipJob{
							FromID: graph.ID(admin),
							ToID:   computerID,
							Kind:   ad.AdminTo,
						}

						if !channels.Submit(ctx, outC, nextJob) {
							return nil
						}
					}

					return nil
				}
			}); err != nil {
				return &analysis.AtomicPostProcessingStats{}, fmt.Errorf("failed submitting reader for operation involving computer %d: %w", computerID, err)
			}

			if err := operation.Operation.SubmitReader(func(ctx context.Context, tx graph.Transaction, outC chan<- analysis.CreatePostRelationshipJob) error {
				if entities, err := FetchCanRDPEntityBitmapForComputer(tx, computerID, threadSafeLocalGroupExpansions, enforceURA, citrixEnabled); err != nil {
					return err
				} else {
					for _, rdp := range entities.Slice() {
						nextJob := analysis.CreatePostRelationshipJob{
							FromID: graph.ID(rdp),
							ToID:   computerID,
							Kind:   ad.CanRDP,
						}

						if !channels.Submit(ctx, outC, nextJob) {
							return nil
						}
					}
				}

				return nil
			}); err != nil {
				return &analysis.AtomicPostProcessingStats{}, fmt.Errorf("failed submitting reader for operation involving computer %d: %w", computerID, err)
			}
		}

		slog.InfoContext(ctx, fmt.Sprintf("Finished post-processing %d active directory computers", computers.GetCardinality()))
		return &operation.Stats, operation.Done()
	}
}

func ExpandLocalGroupMembership(tx graph.Transaction, candidates graph.NodeSet) (graph.NodeSet, error) {
	if paths, err := ExpandLocalGroupMembershipPaths(tx, candidates); err != nil {
		return nil, err
	} else {
		return paths.AllNodes(), nil
	}
}

func ExpandLocalGroupMembershipPaths(tx graph.Transaction, candidates graph.NodeSet) (graph.PathSet, error) {
	groupMemberPaths := graph.NewPathSet()

	for _, candidate := range candidates {
		if candidate.Kinds.ContainsOneOf(ad.Group) {
			if membershipPaths, err := ops.TraversePaths(tx, ops.TraversalPlan{
				Root:      candidate,
				Direction: graph.DirectionInbound,
				BranchQuery: func() graph.Criteria {
					return query.KindIn(query.Relationship(), ad.MemberOf, ad.MemberOfLocalGroup)
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

func Uint64ToIDSlice(uint64IDs []uint64) []graph.ID {
	ids := make([]graph.ID, len(uint64IDs))
	for idx := 0; idx < len(uint64IDs); idx++ {
		ids[idx] = graph.ID(uint64IDs[idx])
	}

	return ids
}

func ExpandGroupMembershipIDBitmap(tx graph.Transaction, group *graph.Node) (*roaring64.Bitmap, error) {
	groupMembers := roaring64.NewBitmap()

	if membershipPaths, err := ops.TraversePaths(tx, ops.TraversalPlan{
		Root:      group,
		Direction: graph.DirectionInbound,
		BranchQuery: func() graph.Criteria {
			return query.Kind(query.Relationship(), ad.MemberOf)
		},
	}); err != nil {
		return nil, err
	} else {
		for _, node := range membershipPaths.AllNodes() {
			groupMembers.Add(node.ID.Uint64())
		}
	}

	return groupMembers, nil
}

func FetchComputerLocalGroupBySIDSuffix(tx graph.Transaction, computer graph.ID, groupSuffix string) (*graph.Node, error) {
	if rel, err := tx.Relationships().Filter(query.And(
		query.StringEndsWith(query.StartProperty(common.ObjectID.String()), groupSuffix),
		query.Kind(query.Relationship(), ad.LocalToComputer),
		query.InIDs(query.EndID(), computer),
	)).First(); err != nil {
		return nil, err
	} else {
		return ops.FetchNode(tx, rel.StartID)
	}
}

func FetchComputerLocalGroupByName(tx graph.Transaction, computer graph.ID, groupName string) (*graph.Node, error) {
	if rel, err := tx.Relationships().Filter(
		query.And(
			query.Kind(query.Start(), ad.LocalGroup),
			query.CaseInsensitiveStringStartsWith(query.StartProperty(common.Name.String()), groupName),
			query.Kind(query.Relationship(), ad.LocalToComputer),
			query.InIDs(query.EndID(), computer),
		),
	).First(); err != nil {
		return nil, err
	} else {
		return ops.FetchNode(tx, rel.StartID)
	}
}

func FetchLocalGroupMembership(tx graph.Transaction, computer graph.ID, groupSuffix string) (graph.NodeSet, error) {
	if localGroup, err := FetchComputerLocalGroupBySIDSuffix(tx, computer, groupSuffix); err != nil {
		return nil, err
	} else {
		return ops.FetchStartNodes(tx.Relationships().Filter(query.And(
			query.KindIn(query.Start(), ad.User, ad.Group, ad.Computer),
			query.Kind(query.Relationship(), ad.MemberOfLocalGroup),
			query.InIDs(query.EndID(), localGroup.ID),
		)))
	}
}

func FetchRemoteInteractiveLogonRightEntities(tx graph.Transaction, computerId graph.ID) (graph.NodeSet, error) {
	return ops.FetchStartNodes(tx.Relationships().Filterf(func() graph.Criteria {
		return query.And(
			query.Kind(query.Relationship(), ad.RemoteInteractiveLogonRight),
			query.Equals(query.EndID(), computerId),
		)
	}))
}

func HasRemoteInteractiveLogonRight(tx graph.Transaction, groupId, computerId graph.ID) bool {
	if _, err := tx.Relationships().Filterf(func() graph.Criteria {
		return query.And(
			query.Equals(query.StartID(), groupId),
			query.Equals(query.EndID(), computerId),
			query.Kind(query.Relationship(), ad.RemoteInteractiveLogonRight),
		)
	}).First(); err != nil {
		return false
	}

	return true
}

func FetchLocalGroupBitmapForComputer(tx graph.Transaction, computer graph.ID, suffix string) (cardinality.Duplex[uint64], error) {
	if members, err := FetchLocalGroupMembership(tx, computer, suffix); err != nil {
		if graph.IsErrNotFound(err) {
			return cardinality.NewBitmap64(), nil
		}

		return nil, err
	} else {
		return graph.NodeSetToDuplex(members), nil
	}
}

func ExpandAllRDPLocalGroups(ctx context.Context, db graph.Database) (impact.PathAggregator, error) {
	slog.InfoContext(ctx, "Expanding all AD group and local group memberships")

	return ResolveAllGroupMemberships(ctx, db, query.Not(
		query.Or(
			query.StringEndsWith(query.StartProperty(common.ObjectID.String()), wellknown.AdministratorsSIDSuffix.String()),
			query.StringEndsWith(query.EndProperty(common.ObjectID.String()), wellknown.AdministratorsSIDSuffix.String()),
		),
	))
}

func FetchCanRDPEntityBitmapForComputer(tx graph.Transaction, computer graph.ID, localGroupExpansions impact.PathAggregator, enforceURA bool, citrixEnabled bool) (cardinality.Duplex[uint64], error) {
	if remoteDesktopUsers, err := FetchRemoteDesktopUsersBitmapForComputer(tx, computer, localGroupExpansions, enforceURA); err != nil {
		return cardinality.NewBitmap64(), err
	} else if remoteDesktopUsers.Cardinality() == 0 || !citrixEnabled {
		return remoteDesktopUsers, nil
	} else {
		// Citrix enabled
		if directAccessUsersGroup, err := FetchComputerLocalGroupByName(tx, computer, "Direct Access Users"); err != nil {
			if graph.IsErrNotFound(err) {
				// "Direct Access Users" is a group that Citrix creates.  If the group does not exist, then the computer does not have Citrix installed and post-processing logic can continue by enumerating the "Remote Desktop Users" AD group.
				return remoteDesktopUsers, nil
			}
			return cardinality.NewBitmap64(), err
		} else {
			if dauGroupMembers, ok := localGroupExpansions.Cardinality(directAccessUsersGroup.ID.Uint64()).(cardinality.Duplex[uint64]); !ok {
				return cardinality.NewBitmap64(), errors.New("type assertion failed in FetchCanRDPEntityBitmapForComputer")
			} else {
				dauGroupMembers.And(remoteDesktopUsers)
				return dauGroupMembers, nil
			}
		}
	}
}

// returns a bitmap containing the ID's of all entities that have RDP privileges to the specified computer via membership to the "Remote Desktop Users" AD group
func FetchRemoteDesktopUsersBitmapForComputer(tx graph.Transaction, computer graph.ID, localGroupExpansions impact.PathAggregator, enforceURA bool) (cardinality.Duplex[uint64], error) {
	if rdpLocalGroup, err := FetchComputerLocalGroupBySIDSuffix(tx, computer, wellknown.RemoteDesktopUsersSIDSuffix.String()); err != nil {
		if graph.IsErrNotFound(err) {
			return cardinality.NewBitmap64(), nil
		}

		return nil, err
	} else if enforceURA || ComputerHasURACollection(tx, computer) {
		return ProcessRDPWithUra(tx, rdpLocalGroup, computer, localGroupExpansions)
	} else if bitmap, err := FetchLocalGroupBitmapForComputer(tx, computer, wellknown.RemoteDesktopUsersSIDSuffix.String()); err != nil {
		return nil, err
	} else {
		return bitmap, nil
	}
}

func ComputerHasURACollection(tx graph.Transaction, computerID graph.ID) bool {
	if computer, err := tx.Nodes().Filterf(func() graph.Criteria {
		return query.Equals(query.NodeID(), computerID)
	}).First(); err != nil {
		return false
	} else {
		if ura, err := computer.Properties.Get(ad.HasURA.String()).Bool(); err != nil {
			return false
		} else {
			return ura
		}
	}
}

func ProcessRDPWithUra(tx graph.Transaction, rdpLocalGroup *graph.Node, computer graph.ID, localGroupExpansions impact.PathAggregator) (cardinality.Duplex[uint64], error) {
	rdpLocalGroupMembers := localGroupExpansions.Cardinality(rdpLocalGroup.ID.Uint64()).(cardinality.Duplex[uint64])
	// Shortcut opportunity: see if the RDP group has RIL privilege. If it does, get the first degree members and return those ids, since everything in RDP group has CanRDP privs. No reason to look any further
	if HasRemoteInteractiveLogonRight(tx, rdpLocalGroup.ID, computer) {
		firstDegreeMembers := cardinality.NewBitmap64()

		return firstDegreeMembers, tx.Relationships().Filter(
			query.And(
				query.Kind(query.Relationship(), ad.MemberOfLocalGroup),
				query.KindIn(query.Start(), ad.Group, ad.User, ad.Computer),
				query.Equals(query.EndID(), rdpLocalGroup.ID),
			),
		).FetchTriples(func(cursor graph.Cursor[graph.RelationshipTripleResult]) error {
			for result := range cursor.Chan() {
				firstDegreeMembers.Add(result.StartID.Uint64())
			}
			return cursor.Error()
		})
	} else if baseRilEntities, err := FetchRemoteInteractiveLogonRightEntities(tx, computer); err != nil {
		return nil, err
	} else {
		var (
			rdpEntities      = cardinality.NewBitmap64()
			secondaryTargets = cardinality.NewBitmap64()
		)

		// Attempt 2: look at each RIL entity directly and see if it has membership to the RDP group. If not, and it's a group, expand its membership for further processing
		for _, entity := range baseRilEntities {
			if rdpLocalGroupMembers.Contains(entity.ID.Uint64()) {
				// If we have membership to the RDP group, then this is a valid CanRDP entity
				rdpEntities.Add(entity.ID.Uint64())
			} else if entity.Kinds.ContainsOneOf(ad.Group, ad.LocalGroup) {
				secondaryTargets.Or(localGroupExpansions.Cardinality(entity.ID.Uint64()).(cardinality.Duplex[uint64]))
			}
		}

		// Attempt 3: Look at each member of expanded groups and see if they have the correct permissions
		for _, entity := range secondaryTargets.Slice() {
			// If we have membership to the RDP group then this is a valid CanRDP entity
			if rdpLocalGroupMembers.Contains(entity) {
				rdpEntities.Add(entity)
			}
		}

		return rdpEntities, nil
	}
}
