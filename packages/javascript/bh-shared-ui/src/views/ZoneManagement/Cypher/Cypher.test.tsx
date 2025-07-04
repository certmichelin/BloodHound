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

import userEvent from '@testing-library/user-event';
import { SeedTypeCypher } from 'js-client-library';
import { rest } from 'msw';
import { setupServer } from 'msw/node';
import { act, render, screen, waitFor } from '../../../test-utils';
import { mockCodemirrorLayoutMethods } from '../../../utils';
import SelectorFormContext, { initialValue } from '../Save/SelectorForm/SelectorFormContext';
import { Cypher } from './Cypher';

const testNodes = {
    members: [
        {
            name: '',
            primary_kind: 'Unknown',
            object_id: '',
        },
    ],
};

const server = setupServer(
    rest.get('/api/v2/graphs/kinds', async (_req, res, ctx) => {
        return res(
            ctx.json({
                data: ['Tier Zero', 'Tier One', 'Tier Two'],
            })
        );
    }),
    rest.post(`/api/v2/asset-group-tags/preview-selectors`, (_, res, ctx) => {
        return res(ctx.json(testNodes));
    })
);

beforeAll(() => server.listen());
afterEach(() => server.resetHandlers());
afterAll(() => {
    server.close();
    vi.restoreAllMocks();
});

describe('Cypher Search component for Zone Management', () => {
    it('renders a preview version', () => {
        render(<Cypher preview />);

        expect(screen.getByText('Cypher Preview')).toBeInTheDocument();
        expect(screen.getByRole('link', { name: 'View in Explore' })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: 'Run' })).not.toBeInTheDocument();
    });

    it('renders a preview version by default', () => {
        render(<Cypher />);

        expect(screen.getByText('Cypher Preview')).toBeInTheDocument();
        expect(screen.getByRole('link', { name: 'View in Explore' })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: 'Run' })).not.toBeInTheDocument();
    });

    test('the input text gets encoded into the "View in Explore" link', () => {
        render(<Cypher initialInput='match(n) return n limit 5' />);

        const link = screen.getByRole('link', { name: 'View in Explore' });

        expect(link).toHaveAttribute(
            'href',
            '/ui/explore?searchType=cypher&exploreSearchTab=cypher&cypherSearch=bWF0Y2gobikgcmV0dXJuIG4gbGltaXQgNQ=='
        );
    });

    it('renders an interactive version when preview is set to false', () => {
        render(<Cypher preview={false} />);

        expect(screen.getByText('Cypher Search')).toBeInTheDocument();
        expect(screen.getByRole('link', { name: 'View in Explore' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Update Sample Results' })).toBeInTheDocument();
    });

    it('runs the query and calls dispatch to set the node results', async () => {
        const user = userEvent.setup();
        const dispatch = vi.fn();
        mockCodemirrorLayoutMethods();

        await act(async () => {
            render(
                <SelectorFormContext.Provider
                    value={{
                        ...initialValue,
                        dispatch,
                    }}>
                    <Cypher preview={false} initialInput='match(n) return n limit 5' />
                </SelectorFormContext.Provider>
            );
        });

        const runButton = screen.getByRole('button', { name: 'Update Sample Results' });

        await user.click(runButton);

        waitFor(() => {
            expect(dispatch).toHaveBeenCalledWith({
                type: 'set-seeds',
                seeds: [{ type: SeedTypeCypher, value: 'match(n) return n limit 5' }],
            });
        });
    });
});
