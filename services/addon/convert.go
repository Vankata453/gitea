// Copyright 2023 Vankata453
// SPDX-License-Identifier: MIT

package addon

import (
	"fmt"
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"encoding/json"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
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

  // Get latest verified release
	release, err := repo_model.GetReleaseForRepoByID(ctx, opts.ID, addonDBInfo.ReleaseID)
	if err != nil {
		return nil, err
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

	// Get api.AddonRepository information for all dependencies
	var dependencies []*api.AddonRepository
	for _, depID := range info.Dependencies {
		// Add-on repository IDs may also be formatted as "{repo_name}_{repo_id}"
		splitID := strings.Split(depID, "_")
		repoID, err := strconv.ParseInt(splitID[len(splitID) - 1], 10, 64)
		if err != nil {
			continue
		}

		repo, err := repo_model.GetRepositoryByID(ctx, repoID)
		if err != nil {
			continue
		}

		depOpts := &AddonRepositoryConvertOptions{
			ID: repo.ID,
			Name: repo.Name,
			OwnerName: repo.OwnerName,
			Topics: repo.Topics,
			Description: repo.Description,
		}
		resultEntry, err := ToAddonRepo(ctx, depOpts)
		if err != nil {
			continue
		}

		dependencies = append(dependencies, resultEntry)
	}

	// Return API add-on repository as a result
	return &api.AddonRepository{
		ID: fmt.Sprintf("%s_%d", opts.Name, opts.ID),
		Version: &api.AddonRepositoryVersion{
			Commit: release.Sha1,
			Title: release.Title,
			Description: release.Note,
			CreatedAt: release.CreatedUnix.AsTime(),
		},
		Type: addonType,
		Title: info.Title,
		Description: opts.Description,
		Author: opts.OwnerName,
		License: info.License,
		OriginURL: opts.HTMLURL(),
		URL: opts.HTMLURL() + "/archive/" + release.Sha1 + ".zip",
		UpstreamURL: fmt.Sprintf("%s/api/v1/repos/addons/%d", strings.TrimSuffix(setting.AppURL, "/"), opts.ID),
		MD5: addonDBInfo.Md5,
		Screenshots: &api.AddonRepositoryScreenshots{
			BaseURL: opts.HTMLURL() + "/raw/commit/" + release.Sha1 + "/screenshots/",
			Files: screenshots,
		},
		Dependencies: dependencies,
	}, nil
}

// ToSexpAddonRepo converts a Repository to api.AddonRepository,
// and afterwards returns the data in an S-Expression add-on index format
func ToSexpAddonRepo(ctx context.Context, opts *AddonRepositoryConvertOptions, indentCount int) (string, error) {
	addonRepo, err := ToAddonRepo(ctx, opts)
	if err != nil {
		return "", err
	}

	return GetSexpAddonRepo(addonRepo, "supertux-addoninfo", indentCount), nil
}

// GetSexpAddonRepo returns the data of an api.AddonRepository in an S-Expression add-on index format.
func GetSexpAddonRepo(addonRepo *api.AddonRepository, headerName string, indentCount int) string {
	indent := strings.Repeat(" ", indentCount)

	// Write an S-Expression-formatted add-on index entry
	var entry string
	entry += indent + "(" + headerName + "\n"
	entry += indent + "  (id \"" + addonRepo.ID + "\")\n"
	entry += indent + "  (version\n"
	entry += indent + "    (commit \"" + addonRepo.Version.Commit + "\")\n"
	entry += indent + "    (title \"" + addonRepo.Version.Title + "\")\n"
	entry += indent + "    (description \"" + addonRepo.Version.Description + "\")\n"
	entry += indent + "    (created-at " + strconv.FormatInt(addonRepo.Version.CreatedAt.Unix(), 10) + ")\n"
	entry += indent + "  )\n"
	entry += indent + "  (type \"" + addonRepo.Type + "\")\n"
	entry += indent + "  (title \"" + addonRepo.Title + "\")\n"
	entry += indent + "  (description \"" + addonRepo.Description + "\")\n"
	entry += indent + "  (author \"" + addonRepo.Author + "\")\n"
	entry += indent + "  (license \"" + addonRepo.License + "\")\n"
	entry += indent + "  (origin-url \"" + addonRepo.OriginURL + "\")\n"
	entry += indent + "  (url \"" + addonRepo.URL + "\")\n"
	entry += indent + "  (upstream-url \"" + addonRepo.UpstreamURL + "\")\n"
	entry += indent + "  (md5 \"" + addonRepo.MD5 + "\")\n"
	if len(addonRepo.Screenshots.Files) > 0 { // Add-on screenshot files are available
		entry += indent + "  (screenshots\n"
		entry += indent + "    (base-url \"" + addonRepo.Screenshots.BaseURL + "\")\n"
		entry += indent + "    (files\n"
		for _, scrFile := range addonRepo.Screenshots.Files { // Print out all screenshot files separately
			entry += indent + "      (file \"" + scrFile + "\")\n"
		}
		entry += indent + "    )\n"
		entry += indent + "  )\n"
	}
	if len(addonRepo.Dependencies) > 0 { // Dependencies are specified
		entry += indent + "  (dependencies\n"
		for _, dependency := range addonRepo.Dependencies { // Print out all dependencies separately
			entry += GetSexpAddonRepo(dependency, "dependency", indentCount + 4) + "\n"
		}
		entry += indent + "  )\n"
	}
	entry += indent + ")"

	return entry
}

// ToSexpAddonIndex combines multiple S-Expression-formatted add-on index entries.
func ToSexpAddonIndex(entries []string, previousPageURL string, nextPageURL string, totalPages int) string {
	var index string
	index += "(supertux-addons\n"
	for _, entry := range entries {
		index += entry + "\n"
	}
	if previousPageURL != "" {
		index += "  (previous-page \"" + previousPageURL + "\")\n"
	}
	if nextPageURL != "" {
		index += "  (next-page \"" + nextPageURL + "\")\n"
	}
	index += "  (total-pages " + strconv.Itoa(totalPages) + ")\n"
	index += ")"

	return index
}
