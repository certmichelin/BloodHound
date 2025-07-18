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
	"sort"
	"strings"
	"time"

	"github.com/specterops/bloodhound/packages/go/analysis/ad/internal/nodeprops"
	"github.com/specterops/bloodhound/packages/go/analysis/ad/wellknown"
	"github.com/specterops/bloodhound/packages/go/analysis/impact"
	"github.com/specterops/bloodhound/packages/go/bhlog/measure"
	"github.com/specterops/bloodhound/packages/go/graphschema/ad"
	"github.com/specterops/bloodhound/packages/go/graphschema/common"
	"github.com/specterops/dawgs/cardinality"
	"github.com/specterops/dawgs/graph"
	"github.com/specterops/dawgs/ops"
	"github.com/specterops/dawgs/query"
	"github.com/specterops/dawgs/util"
)

var (
	AdminGroupSuffix = "-544"
	RDPGroupSuffix   = "-555"
)

const (
	EnterpriseDomainControllersGroupSIDSuffix = "1-5-9"
	AdministratorAccountSIDSuffix             = "-500"
	DomainAdminsGroupSIDSuffix                = "-512"
	DomainUsersSuffix                         = "-513"
	DomainComputersSuffix                     = "-515"
	DomainControllersGroupSIDSuffix           = "-516"
	SchemaAdminsGroupSIDSuffix                = "-518"
	EnterpriseAdminsGroupSIDSuffix            = "-519"
	KeyAdminsGroupSIDSuffix                   = "-526"
	EnterpriseKeyAdminsGroupSIDSuffix         = "-527"
	AdministratorsGroupSIDSuffix              = "-544"
	BackupOperatorsGroupSIDSuffix             = "-551"
	AuthenticatedUsersSuffix                  = "-S-1-5-11"
	EveryoneSuffix                            = "-S-1-1-0"
	ProtectedUsersSuffix                      = "-525"
	AdminSDHolderDNPrefix                     = "CN=ADMINSDHOLDER,CN=SYSTEM,"
)

func TierZeroWellKnownSIDSuffixes() []string {
	return []string{
		EnterpriseDomainControllersGroupSIDSuffix,
		AdministratorAccountSIDSuffix,
		DomainAdminsGroupSIDSuffix,
		DomainControllersGroupSIDSuffix,
		SchemaAdminsGroupSIDSuffix,
		EnterpriseAdminsGroupSIDSuffix,
		KeyAdminsGroupSIDSuffix,
		EnterpriseKeyAdminsGroupSIDSuffix,
		BackupOperatorsGroupSIDSuffix,
		AdministratorsGroupSIDSuffix,
	}
}

func FetchWellKnownTierZeroEntities(ctx context.Context, db graph.Database, domainSID string) (graph.NodeSet, error) {
	defer measure.ContextMeasure(ctx, slog.LevelInfo, "FetchWellKnownTierZeroEntities")()

	nodes := graph.NewNodeSet()

	return nodes, db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		for _, wellKnownSIDSuffix := range TierZeroWellKnownSIDSuffixes() {
			if err := tx.Nodes().Filterf(func() graph.Criteria {
				return query.And(
					// Make sure we have the Group or User label. This should cover the case for URA as well as filter out all the other localgroups
					query.KindIn(query.Node(), ad.Group, ad.User),
					query.StringEndsWith(query.NodeProperty(common.ObjectID.String()), wellKnownSIDSuffix),
					query.Equals(query.NodeProperty(ad.DomainSID.String()), domainSID),
				)
			}).Fetch(func(cursor graph.Cursor[*graph.Node]) error {
				for node := range cursor.Chan() {
					nodes.Add(node)
				}

				return cursor.Error()
			}); err != nil {
				return err
			}
		}

		// AdminSDHolder
		if err := tx.Nodes().Filterf(func() graph.Criteria {
			return query.And(
				query.KindIn(query.Node(), ad.Container),
				query.StringStartsWith(query.NodeProperty(ad.DistinguishedName.String()), AdminSDHolderDNPrefix),
				query.Equals(query.NodeProperty(ad.DomainSID.String()), domainSID),
			)
		}).Fetch(func(cursor graph.Cursor[*graph.Node]) error {
			for node := range cursor.Chan() {
				nodes.Add(node)
			}
			return cursor.Error()
		}); err != nil {
			return err
		}

		return nil
	})
}

func FixWellKnownNodeTypes(ctx context.Context, db graph.Database) error {
	defer measure.ContextMeasure(ctx, slog.LevelInfo, "Fix well known node types")()

	groupSuffixes := []string{
		EnterpriseKeyAdminsGroupSIDSuffix,
		KeyAdminsGroupSIDSuffix,
		EnterpriseDomainControllersGroupSIDSuffix,
		DomainAdminsGroupSIDSuffix,
		DomainControllersGroupSIDSuffix,
		SchemaAdminsGroupSIDSuffix,
		EnterpriseAdminsGroupSIDSuffix,
		AdministratorsGroupSIDSuffix,
		BackupOperatorsGroupSIDSuffix,
	}

	return db.WriteTransaction(ctx, func(tx graph.Transaction) error {
		for _, suffix := range groupSuffixes {
			if nodes, err := ops.FetchNodes(tx.Nodes().Filterf(func() graph.Criteria {
				return query.And(
					query.StringEndsWith(query.NodeProperty(common.ObjectID.String()), suffix),
					query.Not(query.KindIn(query.Node(), ad.Group, ad.LocalGroup)),
				)
			})); err != nil && !graph.IsErrNotFound(err) {
				return err
			} else if graph.IsErrNotFound(err) {
				continue
			} else {
				for _, node := range nodes {
					node.AddKinds(ad.Group)
					if err := tx.UpdateNode(node); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}

func RunDomainAssociations(ctx context.Context, db graph.Database) error {
	defer measure.ContextMeasure(ctx, slog.LevelInfo, "Domain Associations")()

	return db.WriteTransaction(ctx, func(tx graph.Transaction) error {
		if domainNamesByObjectID, err := grabDomainInformation(tx); err != nil {
			return fmt.Errorf("error grabbing domain information for association: %w", err)
		} else if unnamedNodes, err := ops.FetchNodes(tx.Nodes().Filterf(func() graph.Criteria {
			return query.Not(query.Exists(query.NodeProperty(common.Name.String())))
		})); err != nil {
			return fmt.Errorf("error grabbing unnnamed nodes for association: %w", err)
		} else {
			for _, unnamedNode := range unnamedNodes {
				if nodeObjectID, err := unnamedNode.Properties.Get(common.ObjectID.String()).String(); err == nil {
					if objectIDSuffixIdx := strings.LastIndex(nodeObjectID, "-"); objectIDSuffixIdx >= 0 {
						nodeDomainSID := nodeObjectID[:objectIDSuffixIdx]

						if domainName, hasDomain := domainNamesByObjectID[nodeDomainSID]; hasDomain {
							unnamedNode.Properties.Set(common.Name.String(), fmt.Sprintf("(%s) %s", domainName, nodeObjectID))
							unnamedNode.Properties.Set(ad.DomainSID.String(), nodeDomainSID)

							if err := tx.UpdateNode(unnamedNode); err != nil {
								return fmt.Errorf("error renaming nodes during association: %w", err)
							}
						}
					}
				}
			}
		}

		// TODO: Reimplement unassociated node pruning if we decide that FOSS needs unassociated node pruning
		return nil
	})
}

func grabDomainInformation(tx graph.Transaction) (map[string]string, error) {
	domainNamesByObjectID := make(map[string]string)

	if err := tx.Nodes().Filterf(func() graph.Criteria {
		return query.Kind(query.Node(), ad.Domain)
	}).Fetch(func(cursor graph.Cursor[*graph.Node]) error {
		for node := range cursor.Chan() {
			if domainObjectID, err := node.Properties.Get(common.ObjectID.String()).String(); err != nil {
				slog.Error(fmt.Sprintf("Domain node %d does not have a valid object ID", node.ID))
			} else if domainName, err := node.Properties.Get(common.Name.String()).String(); err != nil {
				slog.Error(fmt.Sprintf("Domain node %d does not have a valid name", node.ID))
			} else {
				domainNamesByObjectID[domainObjectID] = domainName
			}
		}

		return cursor.Error()
	}); err != nil {
		return nil, err
	} else {
		return domainNamesByObjectID, nil
	}
}

func LinkWellKnownGroups(ctx context.Context, db graph.Database) error {
	defer measure.ContextMeasure(ctx, slog.LevelInfo, "Link well known groups")()

	var (
		errors        = util.NewErrorCollector()
		newProperties = graph.NewProperties()
	)

	if domains, err := GetCollectedDomains(ctx, db); err != nil {
		return err
	} else {
		newProperties.Set(common.LastSeen.String(), time.Now().UTC())

		for _, domain := range domains {
			// get the domain ID and domain name
			if domainSid, domainName, err := nodeprops.ReadDomainIDandNameAsString(domain); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("Error getting domain sid or name for domain %d: %v", domain.ID, err))
			} else {
				var (
					domainId                = domain.ID
					domainUsersWellKnownSID = wellknown.DefineSID(
						domainSid,
						wellknown.DomainUsersSIDSuffix,
					)
					authUsersIdWellKnownSID = wellknown.DefineSID(
						domainName,
						wellknown.AuthenticatedUsersSIDSuffix,
					)
					everyoneWellKnownSID = wellknown.DefineSID(
						domainName,
						wellknown.EveryoneSIDSuffix,
					)
					domainComputerWellKnownSID = wellknown.DefineSID(
						domainSid,
						wellknown.DomainComputersSIDSuffix,
					)
				)

				if err := db.WriteTransaction(ctx, func(tx graph.Transaction) error {
					if domainUserNode, err := getOrCreateWellKnownGroup(
						tx,
						domainUsersWellKnownSID,
						domainSid,
						domainName,
						wellknown.DefineNodeName(wellknown.DomainUsersNodeNamePrefix, domainName),
					); err != nil {
						return fmt.Errorf("error getting domain users node for domain %d: %w", domainId, err)
					} else if authUsersNode, err := getOrCreateWellKnownGroup(
						tx,
						authUsersIdWellKnownSID,
						domainSid,
						domainName,
						wellknown.DefineNodeName(wellknown.AuthenticatedUsersNodeNamePrefix, domainName),
					); err != nil {
						return fmt.Errorf("error getting auth users node for domain %d: %w", domainId, err)
					} else if everyoneNode, err := getOrCreateWellKnownGroup(
						tx,
						everyoneWellKnownSID,
						domainSid,
						domainName,
						wellknown.DefineNodeName(wellknown.EveryoneNodeNamePrefix, domainName),
					); err != nil {
						return fmt.Errorf("error getting everyone for domain %d: %w", domainId, err)
					} else if domainComputerNode, err := getOrCreateWellKnownGroup(
						tx,
						domainComputerWellKnownSID,
						domainSid,
						domainName,
						wellknown.DefineNodeName(wellknown.DomainComputerNodeNamePrefix, domainName),
					); err != nil {
						return fmt.Errorf("error getting domain computers node for domain %d: %w", domainId, err)
					} else if err := createOrUpdateWellKnownLink(
						tx,
						domainUserNode,
						authUsersNode,
						newProperties,
					); err != nil {
						return err
					} else if err := createOrUpdateWellKnownLink(
						tx,
						domainComputerNode,
						authUsersNode,
						newProperties,
					); err != nil {
						return err
					} else if err := createOrUpdateWellKnownLink(
						tx,
						authUsersNode,
						everyoneNode,
						newProperties,
					); err != nil {
						return err
					} else {
						return nil
					}
				}); err != nil {
					slog.ErrorContext(ctx, fmt.Sprintf(
						"Error linking well known groups for domain %d: %v",
						domain.ID,
						err,
					))
					errors.Add(fmt.Errorf("failed linking well known groups for domain %d: %w", domain.ID, err))
				}
			}
		}

		return errors.Combined()
	}
}

func getOrCreateWellKnownGroup(
	tx graph.Transaction,
	wellKnownSid, domainSid, domainName, nodeName string,
) (
	*graph.Node,
	error,
) {
	// Only filter by ObjectID, not by kind
	if wellKnownNode, err := tx.Nodes().Filterf(func() graph.Criteria {
		return query.Equals(query.NodeProperty(common.ObjectID.String()), wellKnownSid)
	}).First(); err != nil && !graph.IsErrNotFound(err) {
		return nil, err
	} else if graph.IsErrNotFound(err) {
		// Only create a new node if no node with this ObjectID exists
		properties := graph.AsProperties(graph.PropertyMap{
			common.Name:     nodeName,
			common.ObjectID: wellKnownSid,
			ad.DomainSID:    domainSid,
			common.LastSeen: time.Now().UTC(),
			ad.DomainFQDN:   domainName,
		})
		return tx.CreateNode(properties, ad.Entity, ad.Group)
	} else {
		// If a node with this ObjectID exists (regardless of its kind), return it
		// Optionally, we could add the ad.Group kind if it's missing
		if !wellKnownNode.Kinds.ContainsOneOf(ad.Group) {
			// Add the ad.Group kind if it's missing
			wellKnownNode.AddKinds(ad.Group)
			if err := tx.UpdateNode(wellKnownNode); err != nil {
				return nil, fmt.Errorf("failed to update node with Group kind: %w", err)
			}
		}
		return wellKnownNode, nil
	}
}

func createOrUpdateWellKnownLink(
	tx graph.Transaction,
	startNode *graph.Node,
	endNode *graph.Node,
	props *graph.Properties,
) error {
	if rel, err := tx.Relationships().Filterf(func() graph.Criteria {
		return query.And(
			query.Equals(query.StartID(), startNode.ID),
			query.Equals(query.EndID(), endNode.ID),
			query.Kind(query.Relationship(), ad.MemberOf),
		)
	}).First(); err != nil && !graph.IsErrNotFound(err) {
		return err
	} else if graph.IsErrNotFound(err) {
		if _, err := tx.CreateRelationshipByIDs(
			startNode.ID,
			endNode.ID,
			ad.MemberOf,
			props,
		); err != nil {
			return err
		} else {
			return nil
		}
	} else {
		rel.Properties.Set(common.LastSeen.String(), time.Now().UTC())
		return tx.UpdateRelationship(rel)
	}
}

// CalculateCrossProductNodeSets finds the intersection of the given sets of nodes.
// See CalculateCrossProductNodeSetsDoc.md for explaination of the specialGroups (Authenticated Users and Everyone) and why we treat them the way we do
func CalculateCrossProductNodeSets(tx graph.Transaction, groupExpansions impact.PathAggregator, nodeSlices ...[]*graph.Node) cardinality.Duplex[uint64] {
	if len(nodeSlices) < 2 {
		slog.Error("Cross products require at least 2 nodesets")
		return cardinality.NewBitmap64()
	}

	// The intention is that the node sets being passed into this function contain all the first degree principals for control
	var (
		// Temporary storage for first degree and unrolled sets without auth users/everyone
		firstDegreeSets []cardinality.Duplex[uint64]
		unrolledSets    []cardinality.Duplex[uint64]

		// This is the set we use as a reference set to check against checkset
		unrolledRefSet = cardinality.NewBitmap64()

		// This is the set we use to aggregate multiple sets together it should have all the valid principals from all other sets at this point
		checkSet = cardinality.NewBitmap64()

		// This is our set of entities that have the complete cross product of permissions
		resultEntities = cardinality.NewBitmap64()
	)

	// Get the IDs of the Auth. Users and Everyone groups
	specialGroups, err := FetchAuthUsersAndEveryoneGroups(tx)
	if err != nil {
		slog.Error(fmt.Sprintf("Could not fetch groups: %s", err.Error()))
	}

	// Unroll all nodesets
	for _, nodeSlice := range nodeSlices {
		var (
			firstDegreeSet = cardinality.NewBitmap64()
			unrolledSet    = cardinality.NewBitmap64()
		)

		for _, entity := range nodeSlice {
			entityID := entity.ID.Uint64()

			firstDegreeSet.Add(entityID)
			unrolledSet.Add(entityID)

			if entity.Kinds.ContainsOneOf(ad.Group, ad.LocalGroup) {
				unrolledSet.Or(groupExpansions.Cardinality(entity.ID.Uint64()))
			}
		}

		// Skip sets containing Auth. Users or Everyone
		hasSpecialGroup := false

		for _, specialGroup := range specialGroups {
			if unrolledSet.Contains(specialGroup.ID.Uint64()) {
				hasSpecialGroup = true
				break
			}
		}

		if !hasSpecialGroup {
			unrolledSets = append(unrolledSets, unrolledSet)
			firstDegreeSets = append(firstDegreeSets, firstDegreeSet)
		}
	}

	// If every nodeset (unrolled) includes Auth. Users/Everyone then return all nodesets (first degree)
	if len(firstDegreeSets) == 0 {
		for _, nodeSet := range nodeSlices {
			for _, entity := range nodeSet {
				resultEntities.Add(entity.ID.Uint64())
			}
		}

		return resultEntities
	} else if len(firstDegreeSets) == 1 { // If every nodeset (unrolled) except one includes Auth. Users/Everyone then return that one nodeset (first degree)
		return firstDegreeSets[0]
	} else {
		// This means that len(firstDegreeSets) must be greater than or equal to 2 i.e. we have at least two nodesets (unrolled) without Auth. Users/Everyone
		checkSet.Or(unrolledSets[1])

		for _, unrolledSet := range unrolledSets[2:] {
			checkSet.And(unrolledSet)
		}
	}

	// Check first degree principals in our reference set (firstDegreeSets[0]) first
	firstDegreeSets[0].Each(func(id uint64) bool {
		if checkSet.Contains(id) {
			resultEntities.Add(id)
		} else {
			unrolledRefSet.Or(groupExpansions.Cardinality(id))
		}

		return true
	})

	// Find all the groups in our secondary targets and map them to their cardinality in our expansions
	// Saving off to a map to prevent multiple lookups on the expansions
	tempMap := map[uint64]uint64{}
	unrolledRefSet.Each(func(id uint64) bool {
		// If group expansions contains this ID and its cardinality is > 0, it's a group/localgroup
		idCardinality := groupExpansions.Cardinality(id).Cardinality()
		if idCardinality > 0 {
			tempMap[id] = idCardinality
		}

		return true
	})

	// Save the map keys to a new slice, this represents our list of groups in the expansion
	keys := make([]uint64, 0, len(tempMap))

	for key := range tempMap {
		keys = append(keys, key)
	}

	// Sort by cardinality we saved in the map, which will give us all the groups sorted by their number of members
	sort.Slice(keys, func(i, j int) bool {
		return tempMap[keys[i]] < tempMap[keys[j]]
	})

	for _, groupId := range keys {
		// If the set doesn't contain our key, it means that we've already encapsulated this group in a previous shortcut so skip it
		if !unrolledRefSet.Contains(groupId) {
			continue
		}

		if checkSet.Contains(groupId) {
			// If this entity is a cross product, add it to result entities, remove the group id from the second set and xor the group's membership with the result set
			resultEntities.Add(groupId)

			unrolledRefSet.Remove(groupId)
			unrolledRefSet.Xor(groupExpansions.Cardinality(groupId))
		} else {
			// If this isn't a match, remove it from the second set to ensure we don't check it again, but leave its membership
			unrolledRefSet.Remove(groupId)
		}
	}

	unrolledRefSet.Each(func(remainder uint64) bool {
		if checkSet.Contains(remainder) {
			resultEntities.Add(remainder)
		}

		return true
	})

	return resultEntities
}

func GetEdgeCompositionPath(ctx context.Context, db graph.Database, edge *graph.Relationship) (graph.PathSet, error) {
	var (
		err     error
		pathSet = graph.NewPathSet()
	)

	if err = db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		switch edge.Kind {
		case ad.GoldenCert:
			pathSet, err = getGoldenCertEdgeComposition(tx, edge)
		case ad.ADCSESC1:
			pathSet, err = GetADCSESC1EdgeComposition(ctx, db, edge)
		case ad.ADCSESC3:
			pathSet, err = GetADCSESC3EdgeComposition(ctx, db, edge)
		case ad.ADCSESC4:
			pathSet, err = GetADCSESC4EdgeComposition(ctx, db, edge)
		case ad.ADCSESC6a, ad.ADCSESC6b:
			pathSet, err = GetADCSESC6EdgeComposition(ctx, db, edge)
		case ad.ADCSESC9a:
			pathSet, err = GetADCSESC9aEdgeComposition(ctx, db, edge)
		case ad.ADCSESC9b:
			pathSet, err = GetADCSESC9bEdgeComposition(ctx, db, edge)
		case ad.ADCSESC10a, ad.ADCSESC10b:
			pathSet, err = GetADCSESC10EdgeComposition(ctx, db, edge)
		case ad.ADCSESC13:
			pathSet, err = GetADCSESC13EdgeComposition(ctx, db, edge)
		case ad.CoerceAndRelayNTLMToADCS:
			pathSet, err = GetCoerceAndRelayNTLMtoADCSEdgeComposition(ctx, db, edge)
		case ad.CoerceAndRelayNTLMToSMB:
			pathSet, err = GetCoerceAndRelayNTLMtoSMBEdgeComposition(ctx, db, edge)
		case ad.GPOAppliesTo:
			pathSet, err = GetGPOAppliesToComposition(ctx, db, edge)
		case ad.CanApplyGPO:
			pathSet, err = GetCanApplyGPOComposition(ctx, db, edge)

		}
		return err
	}); err != nil {
		return graph.NewPathSet(), err
	}
	return pathSet, nil
}

func GetRelayTargets(ctx context.Context, db graph.Database, edge *graph.Relationship) (graph.NodeSet, error) {
	var (
		err     error
		nodeSet = graph.NewNodeSet()
	)

	if err = db.ReadTransaction(ctx, func(tx graph.Transaction) error {
		switch edge.Kind {
		case ad.CoerceAndRelayNTLMToLDAP:
			nodeSet, err = GetVulnerableDomainControllersForRelayNTLMtoLDAP(ctx, db, edge)
		case ad.CoerceAndRelayNTLMToLDAPS:
			nodeSet, err = GetVulnerableDomainControllersForRelayNTLMtoLDAPS(ctx, db, edge)
		case ad.CoerceAndRelayNTLMToADCS:
			nodeSet, err = GetVulnerableEnterpriseCAsForRelayNTLMtoADCS(ctx, db, edge)
		case ad.CoerceAndRelayNTLMToSMB:
			nodeSet, err = GetCoercionTargetsForCoerceAndRelayNTLMtoSMB(ctx, db, edge)
		}
		return err
	}); err != nil {
		return graph.NewNodeSet(), err
	}
	return nodeSet, nil
}
