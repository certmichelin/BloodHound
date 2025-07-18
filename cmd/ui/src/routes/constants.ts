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

export const ROUTE_HOME = '/';
export const ROUTE_EXPLORE = '/explore';
export const ROUTE_GROUP_MANAGEMENT = '/group-management';
export const ROUTE_ZONE_MANAGEMENT = '/zone-management/';
export const ROUTE_ZONE_MANAGEMENT_ROOT = ROUTE_ZONE_MANAGEMENT + '*';
export const ROUTE_LOGIN = '/login';
export const ROUTE_CHANGE_PASSWORD = '/changepassword';
export const ROUTE_USER_DISABLED = '/user-disabled';
export const ROUTE_TWO_FACTOR_AUTHENTICATION = '/login-2fa';
export const ROUTE_EXPIRED_PASSWORD = '/expired-password';
export const ROUTE_MY_PROFILE = '/my-profile';
export const ROUTE_DOWNLOAD_COLLECTORS = '/download-collectors';
export const ROUTE_ADMINISTRATION = '/administration/';
export const ROUTE_ADMINISTRATION_ROOT = ROUTE_ADMINISTRATION + '*';
export const ROUTE_ADMINISTRATION_FILE_INGEST = ROUTE_ADMINISTRATION + 'file-ingest';
export const ROUTE_ADMINISTRATION_DATA_QUALITY = ROUTE_ADMINISTRATION + 'data-quality';
export const ROUTE_ADMINISTRATION_DB_MANAGEMENT = ROUTE_ADMINISTRATION + 'database-management';
export const ROUTE_ADMINISTRATION_MANAGE_USERS = ROUTE_ADMINISTRATION + 'manage-users';
export const ROUTE_ADMINISTRATION_SSO_CONFIGURATION = ROUTE_ADMINISTRATION + 'sso-configuration';
export const ROUTE_ADMINISTRATION_EARLY_ACCESS_FEATURES = ROUTE_ADMINISTRATION + 'early-access-features';
export const ROUTE_ADMINISTRATION_BLOODHOUND_CONFIGURATION = ROUTE_ADMINISTRATION + 'bloodhound-configuration';
export const ROUTE_API_EXPLORER = '/api-explorer';

export const ENVIRONMENT_SUPPORTED_ROUTES = [ROUTE_GROUP_MANAGEMENT, ROUTE_ADMINISTRATION_DATA_QUALITY];
export const DEFAULT_ADMINISTRATION_ROUTE = ROUTE_ADMINISTRATION_FILE_INGEST;
