// Copyright 2023 Vankata453
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"

	"xorm.io/builder"
)

const (
	ADDON_TYPE_WORLDMAP         string = "worldmap"
	ADDON_TYPE_WORLD            string = "world"
	ADDON_TYPE_LEVELSET         string = "levelset"
	ADDON_TYPE_LANGUAGEPACK     string = "languagepack"
	ADDON_TYPE_RESOURCEPACK     string = "resourcepack"
	ADDON_TYPE_WEAKRESOURCEPACK string = "weakresourcepack"
)

func IsValidAddonType(t string) bool {
	tl := strings.ToLower(t)
	return tl == ADDON_TYPE_WORLDMAP ||
		tl == ADDON_TYPE_WORLD ||
		tl == ADDON_TYPE_LEVELSET ||
		tl == ADDON_TYPE_LANGUAGEPACK ||
		tl == ADDON_TYPE_RESOURCEPACK ||
		tl == ADDON_TYPE_WEAKRESOURCEPACK;
}

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
