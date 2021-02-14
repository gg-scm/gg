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

CREATE TABLE "commits" (
	"revno" INTEGER NOT NULL PRIMARY KEY
		CHECK ("revno" >= 0),
	"sha1sum" BLOB NOT NULL UNIQUE CHECK (length("sha1sum") = 20),

	"message" TEXT NOT NULL,
	"author" TEXT NOT NULL,
	"author_date" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	"author_tzoffset" INTEGER NOT NULL DEFAULT 0,
	"committer" TEXT NOT NULL,
	"commit_date" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	"commit_tzoffset" INTEGER NOT NULL DEFAULT 0,

	"gpg_signature" TEXT
		CHECK ("gpg_signature" IS NULL OR
			substr("gpg_signature", -1) = char(10)),

	"tree_sha1sum" BLOB NOT NULL CHECK (length("sha1sum") = 20)
);

CREATE TABLE "commit_parents" (
	"revno" INTEGER NOT NULL
		REFERENCES "commits"
			ON DELETE CASCADE
			ON UPDATE CASCADE,
	"index" INTEGER NOT NULL
		DEFAULT 0
		CHECK ("index" >= 0),
	"parent_revno" INTEGER NOT NULL
		REFERENCES "commits"
		CHECK ("parent_revno" < "revno"),

	PRIMARY KEY ("revno", "index"),
	UNIQUE ("revno", "parent_revno")
);

CREATE TABLE "tag_objects" (
	"id" INTEGER NOT NULL PRIMARY KEY,
	"sha1sum" BLOB NOT NULL UNIQUE CHECK (length("sha1sum") = 20),

	"object_sha1sum" BLOB NOT NULL CHECK (length("sha1sum") = 20),
	"object_type" TEXT NOT NULL
		DEFAULT 'commit'
		CHECK ("object_type" = 'blob' OR
			"object_type" = 'tree' OR
			"object_type" = 'commit' OR
			"object_type" = 'tag'),
	"name" TEXT NOT NULL,
	"tagger" TEXT NOT NULL,
	"tag_date" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	"tag_tzoffset" INTEGER NOT NULL DEFAULT 0,
	"message" TEXT NOT NULL
);
