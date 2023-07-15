// Copyright 2023 Vankata453
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"xorm.io/xorm"

	addon_repo_model "code.gitea.io/gitea/models/repo_addon"
)

func AddAddonRepositoryTable(x *xorm.Engine) error {
	return x.Sync2(new(addon_repo_model.AddonRepository))
}
