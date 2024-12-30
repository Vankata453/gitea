// Copyright 2023-2024 Vankata453

package v1_23 //nolint

import (
	"xorm.io/xorm"

	addon_repo_model "code.gitea.io/gitea/models/repo_addon"
)

func AddAddonRepositoryTable(x *xorm.Engine) error {
	return x.Sync2(new(addon_repo_model.AddonRepository))
}
