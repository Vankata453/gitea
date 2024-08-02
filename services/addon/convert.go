// Copyright 2023 Vankata453
// SPDX-License-Identifier: MIT

package addon

import (
	"fmt"
	"context"
	"errors"
	"net/url"
	"strings"
	"encoding/json"

	"code.gitea.io/gitea/models/db"
	addon_repo_model "code.gitea.io/gitea/models/repo_addon"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

// AddonRepositoryConvertOptions represents options, provided when converting a Repository to api.AddonRepository
type AddonRepositoryConvertOptions struct {
	ID          int64
	Name        string
	OwnerName   string
	Topics      []string
	Description string
}

// HTMLURL returns the repository HTML URL
func (opts *AddonRepositoryConvertOptions) HTMLURL() string {
	return setting.AppURL + url.PathEscape(opts.OwnerName) + "/" + url.PathEscape(opts.Name)
}

// ToAddonRepo converts a Repository to api.AddonRepository
func ToAddonRepo(ctx context.Context, opts *AddonRepositoryConvertOptions) (*api.AddonRepository, error) {
	// Load saved data for the add-on repository from the database
	addonDBInfo := &addon_repo_model.AddonRepository{
		RepoID: opts.ID,
	}
	hasDBInfo, err := db.GetEngine(ctx).Get(addonDBInfo)
	if err != nil {
		return nil, err
	}
	if !hasDBInfo {
		return nil, errors.New("No database information for add-on repository \"" + opts.Name + "\".")
	}
	if len(addonDBInfo.VerifiedCommits) == 0 {
		return nil, errors.New("No verified commits available for add-on repository \"" + opts.Name + "\".")
	}

	// Parse the "info" file
	var info api.AddonRepositoryInfo
	err_ := json.Unmarshal([]byte(addonDBInfo.InfoFile), &info)
	if err_ != nil {
		return nil, err_
	}

	// Get type from topics, if available
	var addonType = "worldmap" // Default type
	for _, topic := range opts.Topics {
		if topic == "world" || topic == "levelset" ||
				topic == "languagepack" || topic == "resourcepack" || topic == "addon" {
			addonType = topic
			break
		}
	}

	// List all screenshots
	screenshots := strings.Split(addonDBInfo.Screenshots, "/")
	if len(screenshots) == 1 && screenshots[0] == "" {
		screenshots = nil
	}

	// Return API add-on repository as a result
	return &api.AddonRepository{
		ID: fmt.Sprintf("%s_%d", opts.Name, opts.ID),
		Versions: addonDBInfo.VerifiedCommits,
		Type: addonType,
		Title: info.Title,
		Description: opts.Description,
		Author: opts.OwnerName,
		License: info.License,
		URL: opts.HTMLURL() + "/archive/" + addonDBInfo.VerifiedCommits[0] + ".zip",
		UpdateURL: fmt.Sprintf("%s/api/v1/repos/addons/%d", strings.TrimSuffix(setting.AppURL, "/"), opts.ID),
		MD5: addonDBInfo.Md5,
		Screenshots: &api.AddonRepositoryScreenshots{
			BaseURL: opts.HTMLURL() + "/raw/commit/" + addonDBInfo.VerifiedCommits[0] + "/screenshots/",
			Files: screenshots,
		},
		Dependencies: info.Dependencies,
	}, nil
}

// ToSexpAddonRepo converts a Repository to api.AddonRepository,
// and afterwards returns the data in an S-Expression add-on index format
func ToSexpAddonRepo(ctx context.Context, opts *AddonRepositoryConvertOptions) (string, error) {
	addonRepo, err := ToAddonRepo(ctx, opts)
	if err != nil {
		return "", err
	}

	// Write an S-Expression-formatted add-on index entry
	var entry string
	entry += "(supertux-addoninfo\n"
	entry += "  (id \"" + addonRepo.ID + "\")\n"
	entry += "  (version \"" + addonRepo.Versions[0] + "\")\n"
	entry += "  (type \"" + addonRepo.Type + "\")\n"
	entry += "  (title \"" + addonRepo.Title + "\")\n"
	entry += "  (description \"" + addonRepo.Description + "\")\n"
	entry += "  (author \"" + addonRepo.Author + "\")\n"
	entry += "  (license \"" + addonRepo.License + "\")\n"
	entry += "  (url \"" + addonRepo.URL + "\")\n"
	entry += "  (update-url \"" + addonRepo.UpdateURL + "\")\n"
	entry += "  (md5 \"" + addonRepo.MD5 + "\")\n"
	if len(addonRepo.Screenshots.Files) > 0 { // Add-on screenshot files are available
		entry += "  (screenshots\n"
		entry += "    (base-url \"" + addonRepo.Screenshots.BaseURL + "\")\n"
		entry += "    (files\n"
		for _, scrFile := range addonRepo.Screenshots.Files { // Print out all screenshot files separately
			entry += "      (file \"" + scrFile + "\")\n"
		}
		entry += "    )\n"
		entry += "  )\n"
	}
	if len(addonRepo.Dependencies) > 0 { // Dependencies are specified
		entry += "  (dependencies\n"
		for _, depID := range addonRepo.Dependencies { // Print out all screenshot files separately
			entry += "    (dependency \"" + depID + "\")\n"
		}
		entry += "  )\n"
	}
	entry += ")"

	return entry, nil
}

// ToSexpAddonIndex combines multiple S-Expression-formatted add-on index entries.
func ToSexpAddonIndex(entries []string) string {
	var index string
	index += "(supertux-addons\n"
	for _, entry := range entries {
		index += entry + "\n"
	}
	index += ")"

	return index
}
