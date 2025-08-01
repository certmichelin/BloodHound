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

import { Typography } from '@mui/material';
import { FC } from 'react';

const Abuse: FC = () => {
    return (
        <>
            <Typography variant='body2'>
                Control over the container node DACL enables an attacker to compromise the child node by creating ACEs,
                such as GenericAll, with inheritance enabled, allowing those permissions to apply to the child node.
            </Typography>
            <Typography variant='body2'>Refer to the inbound edges on the container node for more details.</Typography>
        </>
    );
};

export default Abuse;
