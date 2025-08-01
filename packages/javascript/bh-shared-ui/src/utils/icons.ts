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

import { library } from '@fortawesome/fontawesome-svg-core';
import {
    IconDefinition,
    faArrowsLeftRightToLine,
    faBolt,
    faBox,
    faBoxOpen,
    faBuilding,
    faClipboardCheck,
    faClipboardList,
    faCloud,
    faCog,
    faCube,
    faCubes,
    faDesktop,
    faGem,
    faGlobe,
    faIdCard,
    faKey,
    faLandmark,
    faList,
    faLock,
    faMinus,
    faObjectGroup,
    faPlus,
    faQuestion,
    faRobot,
    faServer,
    faSitemap,
    faSkull,
    faStore,
    faUser,
    faUsers,
    faWindowRestore,
    fas,
} from '@fortawesome/free-solid-svg-icons';
import { ActiveDirectoryNodeKind, AzureNodeKind } from '../graphSchema';

library.add(fas);

export type IconInfo = {
    icon: IconDefinition;
    color: string;
    url?: string;
};

export type IconDictionary = {
    [index: string]: IconInfo;
};

export type GlyphIconInfo = IconInfo & { iconColor: string };

export type GlyphDictionary = {
    [index: string]: GlyphIconInfo;
};

export enum GlyphKind {
    TIER_ZERO,
    TIER_ZERO_DARK,
    OWNED_OBJECT,
    OWNED_OBJECT_DARK,
    EXPAND,
    COLLAPSE,
}

export const NODE_ICON: IconDictionary = {
    [ActiveDirectoryNodeKind.User]: {
        icon: faUser,
        color: '#17E625',
    },

    [ActiveDirectoryNodeKind.Group]: {
        icon: faUsers,
        color: '#DBE617',
    },

    [ActiveDirectoryNodeKind.Computer]: {
        icon: faDesktop,
        color: '#E67873',
    },

    [ActiveDirectoryNodeKind.Domain]: {
        icon: faGlobe,
        color: '#17E6B9',
    },

    [ActiveDirectoryNodeKind.GPO]: {
        icon: faList,
        color: '#998EFD',
    },

    [ActiveDirectoryNodeKind.AIACA]: {
        icon: faArrowsLeftRightToLine,
        color: '#9769F0',
    },

    [ActiveDirectoryNodeKind.RootCA]: {
        icon: faLandmark,
        color: '#6968E8',
    },

    [ActiveDirectoryNodeKind.EnterpriseCA]: {
        icon: faBuilding,
        color: '#4696E9',
    },

    [ActiveDirectoryNodeKind.NTAuthStore]: {
        icon: faStore,
        color: '#D575F5',
    },

    [ActiveDirectoryNodeKind.CertTemplate]: {
        icon: faIdCard,
        color: '#B153F3',
    },

    [ActiveDirectoryNodeKind.IssuancePolicy]: {
        icon: faClipboardCheck,
        color: '#99B2DD',
    },

    [ActiveDirectoryNodeKind.OU]: {
        icon: faSitemap,
        color: '#FFAA00',
    },

    [ActiveDirectoryNodeKind.Container]: {
        icon: faBox,
        color: '#F79A78',
    },

    [AzureNodeKind.User]: {
        icon: faUser,
        color: '#34D2EB',
    },

    [AzureNodeKind.Group]: {
        icon: faUsers,
        color: '#F57C9B',
    },

    [AzureNodeKind.Group365]: {
        icon: faUsers,
        color: '#34D2EB',
    },

    [AzureNodeKind.Tenant]: {
        icon: faCloud,
        color: '#54F2F2',
    },

    [AzureNodeKind.Subscription]: {
        icon: faKey,
        color: '#D2CCA1',
    },

    [AzureNodeKind.ResourceGroup]: {
        icon: faCube,
        color: '#89BD9E',
    },

    [AzureNodeKind.VM]: {
        icon: faDesktop,
        color: '#F9ADA0',
    },
    [AzureNodeKind.WebApp]: {
        icon: faObjectGroup,
        color: '#4696E9',
    },
    [AzureNodeKind.LogicApp]: {
        icon: faSitemap,
        color: '#9EE047',
    },

    [AzureNodeKind.AutomationAccount]: {
        icon: faCog,
        color: '#F4BA44',
    },

    [AzureNodeKind.FunctionApp]: {
        icon: faBolt,
        color: '#F4BA44',
    },

    [AzureNodeKind.ContainerRegistry]: {
        icon: faBoxOpen,
        color: '#0885D7',
    },

    [AzureNodeKind.ManagedCluster]: {
        icon: faCubes,
        color: '#326CE5',
    },

    [AzureNodeKind.Device]: {
        icon: faDesktop,
        color: '#B18FCF',
    },

    [AzureNodeKind.KeyVault]: {
        icon: faLock,
        color: '#ED658C',
    },

    [AzureNodeKind.App]: {
        icon: faWindowRestore,
        color: '#03FC84',
    },

    [AzureNodeKind.VMScaleSet]: {
        icon: faServer,
        color: '#007CD0',
    },

    [AzureNodeKind.ServicePrincipal]: {
        icon: faRobot,
        color: '#C1D6D6',
    },

    [AzureNodeKind.Role]: {
        icon: faClipboardList,
        color: '#ED8537',
    },

    [AzureNodeKind.ManagementGroup]: {
        icon: faSitemap,
        color: '#BD93D8',
    },
};

export const GLYPHS: GlyphDictionary = {
    [GlyphKind.TIER_ZERO]: {
        icon: faGem,
        color: '#000000',
        iconColor: '#FFFFFF',
    },
    [GlyphKind.TIER_ZERO_DARK]: {
        icon: faGem,
        color: '#FFFFFF',
        iconColor: '#000000',
    },
    [GlyphKind.OWNED_OBJECT]: {
        icon: faSkull,
        color: '#000000',
        iconColor: '#FFFFFF',
    },
    [GlyphKind.OWNED_OBJECT_DARK]: {
        icon: faSkull,
        color: '#FFFFFF',
        iconColor: '#000000',
    },
    [GlyphKind.EXPAND]: {
        icon: faPlus,
        color: '#FFFFFF',
        iconColor: '#000000',
    },
    [GlyphKind.COLLAPSE]: {
        icon: faMinus,
        color: '#FFFFFF',
        iconColor: '#000000',
    },
};

export const UNKNOWN_ICON: IconInfo = {
    icon: faQuestion,
    color: '#FFFFFF',
};

export const DEFAULT_ICON_BACKGROUND = '#FFFFFF';

/**
 * Returns icon metadata for a given icon name.
 *
 * This function checks the user-provided customIcons dictionary first,
 * allowing users to override the default icon definitions. If no override
 * is found, it falls back to the built-in NODE_ICON map. If the icon name
 * is unrecognized, a question-mark fallback icon is returned.
 */
export const GetIconInfo = (iconName: string, customIcons: IconDictionary): IconInfo => {
    if (iconName in customIcons) {
        return customIcons[iconName];
    }

    if (iconName in NODE_ICON) {
        return NODE_ICON[iconName];
    }

    return UNKNOWN_ICON;
};
