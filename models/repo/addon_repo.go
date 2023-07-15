// Copyright 2023 Vankata453
// SPDX-License-Identifier: MIT

package repo

import (
	"xorm.io/builder"
)

// Filter out only non-empty regular public repositories, which are not by the "supertux" organization.
func IsAddonRepository(repo *Repository) bool {
	return !(repo.IsTemplate || repo.IsPrivate || repo.IsFork || repo.IsMirror || repo.IsEmpty || repo.OwnerName == "supertux")
}
func AddonRepositoryCondition(cond builder.Cond) builder.Cond {
	return cond.And(
		builder.Eq{"is_template": false},
		builder.Eq{"is_private": false},
		builder.Eq{"is_fork": false},
		builder.Eq{"is_mirror": false},
		builder.Eq{"is_empty": false},
		builder.Not{builder.Eq{"owner_name": "supertux"}},
	)
}
