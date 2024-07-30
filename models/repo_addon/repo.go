// Copyright 2023 Vankata453
// SPDX-License-Identifier: MIT

package repo_addon

// AddonRepository represents saved data for add-on repositories
type AddonRepository struct {
	ID              int64    `xorm:"pk autoincr"`
	RepoID          int64    `xorm:"index unique(s)"`
	VerifiedCommits []string `xorm:"TEXT JSON"`
	InfoFile        string   `xorm:"TEXT JSON"`
	Md5             string   `xorm:"TEXT"`
	Screenshots     string   `xorm:"TEXT"`
}
