-- Copyright 2021 The gg Authors
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
--     https://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.
--
-- SPDX-License-Identifier: Apache-2.0

create temporary table "pack_objects" (
  "offset" integer not null primary key,
  "type",
  "data",
  "sha1sum"
);
create index "temp"."pack_by_type" on "pack_objects" ("type");
create index "temp"."pack_by_sha1" on "pack_objects" ("sha1sum");
create temporary table "pack_deltas" (
	"offset" integer not null primary key,
	"base_offset",
	"base_object",
	"delta_data"
);
