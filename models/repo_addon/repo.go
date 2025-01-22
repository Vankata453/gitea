// Copyright 2023 Vankata453
// SPDX-License-Identifier: MIT

package repo_addon

// AddonRepository represents saved data for add-on repositories
type AddonRepository struct {
	ID           int64    `xorm:"pk autoincr"`
	RepoID       int64    `xorm:"index unique(s)"`
	ReleaseID    int64    `xorm:"index not null"`
	Title        string   `xorm:"TEXT"`
	Description  string   `xorm:"TEXT"`
	Type         string   `xorm:"TEXT"`
	License      string   `xorm:"TEXT"`
	Dependencies string   `xorm:"TEXT"`
	Md5          string   `xorm:"TEXT"`
	Screenshots  string   `xorm:"TEXT"`
}
