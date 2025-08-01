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

import { Screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AssetGroupMemberCountsResponse, AssetGroupMemberParams } from 'js-client-library';
import { rest } from 'msw';
import { setupServer } from 'msw/node';
import { ActiveDirectoryNodeKind } from '../../graphSchema';
import { createMockAssetGroupMemberParams, createMockMemberCounts } from '../../mocks/factories';
import { act, render } from '../../test-utils';
import AssetGroupFilters, { FILTERABLE_PARAMS } from './AssetGroupFilters';

const filterParams = createMockAssetGroupMemberParams();
const memberCounts = createMockMemberCounts();

describe('AssetGroupEdit', () => {
    const server = setupServer(
        rest.get(`/api/v2/custom-nodes`, async (req, res, ctx) => {
            return res(
                ctx.json({
                    data: [],
                })
            );
        })
    );

    beforeAll(() => server.listen());
    afterEach(() => server.resetHandlers());
    afterAll(() => server.close());

    const setup = async (options?: {
        filterParams?: AssetGroupMemberParams;
        memberCounts?: AssetGroupMemberCountsResponse['data'];
    }) => {
        const user = userEvent.setup();
        const handleFilterChange = vi.fn();
        const screen: Screen = await act(async () => {
            return render(
                <AssetGroupFilters
                    filterParams={options?.filterParams ?? {}}
                    handleFilterChange={handleFilterChange}
                    memberCounts={memberCounts}
                />
            );
        });
        return { user, screen, handleFilterChange };
    };

    it('renders a button that expands the filter section', async () => {
        const { screen, user } = await setup({ filterParams, memberCounts });
        const filtersButton = screen.getByTestId('display-filters-button');
        const collapsedSection = screen.getByTestId('asset-group-filter-collapsible-section');

        expect(filtersButton).toBeInTheDocument();
        expect(collapsedSection.classList.contains('MuiCollapse-hidden')).toBeTruthy();

        await user.click(filtersButton);

        const expandedSection = screen.getByTestId('asset-group-filter-collapsible-section');
        // we need to wait a moment while MUI runs the animation to expand this section
        await waitFor(() => expect(expandedSection.classList.contains('MuiCollapse-entered')).toBeTruthy());
    });

    it('indicates that filters are active', async () => {
        const { screen } = await setup({ filterParams, memberCounts });

        const activeFiltersDot = screen.getByTestId('active-filters-dot');

        expect(activeFiltersDot).toHaveStyle({ visibility: 'visible' });
    });

    it('indicates that filters are inactive', async () => {
        const { screen } = await setup();

        const activeFiltersDot = screen.getByTestId('active-filters-dot');

        expect(activeFiltersDot).not.toHaveStyle({ visibility: 'visible' });
    });

    describe('Node Type dropdown filter', () => {
        it('displays the value from filterParams.node_type', async () => {
            const { screen } = await setup({ filterParams, memberCounts });
            const nodeTypeFilter = screen.getByTestId('asset-groups-node-type-filter');
            const nodeTypeFilterValue = nodeTypeFilter.firstChild?.nextSibling;

            expect(nodeTypeFilter.textContent).toContain('Domain');
            expect((nodeTypeFilterValue as HTMLInputElement)?.value).toBe('eq:Domain');
        });

        it('lists all available node kinds as options to filter by', async () => {
            const { screen, user } = await setup({ memberCounts });

            await user.click(screen.getByTestId('display-filters-button'));
            await user.click(screen.getByLabelText('Node Type'));

            const nodeKindList = await screen.findAllByRole('option');

            expect(nodeKindList).toHaveLength(memberCounts.total_count + 1); // +1 for the default empty value

            for (const nodeKind in memberCounts.counts) {
                expect(screen.getByText(nodeKind)).toBeInTheDocument();
            }
        });

        it('calls handleFilterChange when a node type is selected', async () => {
            const { screen, user, handleFilterChange } = await setup({ memberCounts });

            const expectedNodeKind = ActiveDirectoryNodeKind.Domain;

            await user.click(screen.getByTestId('display-filters-button'));
            await user.click(screen.getByLabelText('Node Type'));
            await user.click(screen.getByText(expectedNodeKind));

            expect(handleFilterChange).toBeCalledTimes(1);
            expect(handleFilterChange).toHaveBeenCalledWith('primary_kind', `eq:${expectedNodeKind}`);
        });
    });

    describe('Custom Member checkbox filter', () => {
        it("displays the checkbox as checked if the filter params value is 'true'", async () => {
            const { screen } = await setup({ filterParams: { custom_member: 'eq:true' }, memberCounts });
            const checkbox = screen.getByTestId('asset-groups-custom-member-filter');

            expect((checkbox.firstChild as HTMLInputElement)?.checked).toBe(true);
        });

        it('invokes handleFilterChange with eq:false when clicked and custom_member filter is on', async () => {
            const { screen, user, handleFilterChange } = await setup({ filterParams, memberCounts });
            const checkbox = screen.getByTestId('asset-groups-custom-member-filter');

            await user.click(checkbox);

            expect(handleFilterChange).toBeCalledTimes(1);
            expect(handleFilterChange).toBeCalledWith('custom_member', 'eq:false');
        });

        it('invokes handleFilterChange with eq:true when clicked and custom_member filter is off', async () => {
            const { screen, user, handleFilterChange } = await setup();
            const checkbox = screen.getByTestId('asset-groups-custom-member-filter');

            await user.click(checkbox);

            expect(handleFilterChange).toBeCalledTimes(1);
            expect(handleFilterChange).toBeCalledWith('custom_member', 'eq:true');
        });
    });

    describe('Clear Filters button', () => {
        it('has a button with text Clear Filters', async () => {
            const { screen } = await setup({ filterParams, memberCounts });
            const clearFilersButton = screen.getByText('Clear Filters');

            expect(clearFilersButton).toBeInTheDocument();
        });

        it('calls handleFilterChange with all filter types and empty strings when clicked while filters are active', async () => {
            const { screen, user, handleFilterChange } = await setup({ filterParams, memberCounts });
            const clearFilersButton = screen.getByText('Clear Filters');

            await user.click(clearFilersButton);

            expect(handleFilterChange).toBeCalledTimes(FILTERABLE_PARAMS.length);
            FILTERABLE_PARAMS.forEach((filter) => {
                expect(handleFilterChange).toBeCalledWith(filter, '');
            });
        });

        it('is disabled if no filters are active', async () => {
            const { screen } = await setup();
            const clearFilersButton: HTMLButtonElement = screen.getByText('Clear Filters');

            expect(clearFilersButton.disabled).toBe(true);
        });
    });
});
