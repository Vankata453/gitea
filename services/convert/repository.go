// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"errors"
	"context"
	"time"
	"strings"
	"encoding/hex"
	"encoding/json"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	addon_repo_model "code.gitea.io/gitea/models/repo_addon"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/log"
)

// ToRepo converts a Repository to api.Repository
func ToRepo(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode) *api.Repository {
	return innerToRepo(ctx, repo, mode, false)
}

// ToAddonRepo converts a Repository to api.AddonRepository
func ToAddonRepo(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode) (*api.AddonRepository, error) {
	return innerToAddonRepo(ctx, repo, mode, false)
}

// ToSexpAddonRepo converts a Repository to api.AddonRepository,
// and afterwards returns the data in an S-Expression add-on index format
func ToSexpAddonRepo(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode) (string, error) {
	addonRepo, err := ToAddonRepo(ctx, repo, mode)
	if err != nil {
		return "", err
	}

	// Write an S-Expression-formatted add-on index entry
	var entry string
	entry += "(supertux-addoninfo\n"
	entry += "  (id \"" + addonRepo.ID + "\")\n"
	entry += "  (version \"" + addonRepo.Version + "\")\n"
	entry += "  (type \"" + addonRepo.Type + "\")\n"
	entry += "  (title \"" + addonRepo.Title + "\")\n"
	entry += "  (description \"" + addonRepo.Description + "\")\n"
	entry += "  (author \"" + addonRepo.Author + "\")\n"
	entry += "  (license \"" + addonRepo.License + "\")\n"
	entry += "  (url \"" + addonRepo.URL + "\")\n"
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

func innerToAddonRepo(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode, isParent bool) (*api.AddonRepository, error) {
	// Open the repository
	gitRepo, err := git.OpenRepository(git.DefaultContext, repo_model.RepoPath(repo.OwnerName, repo.Name))
	if err != nil {
		return nil, err
	}
	// Close the repository
	defer gitRepo.Close()

	// Get the default branch
	branch, err := gitRepo.GetBranch(repo.DefaultBranch)
	if err != nil {
		return nil, err
	}

	// Get last commit on the repository's default branch
	commit, err := branch.GetCommit()
	if err != nil {
		return nil, err
	}
	commitID := hex.EncodeToString(commit.ID[:])

	// Load saved data for the add-on repository from the database
	addonDBInfo := &addon_repo_model.AddonRepository{
		RepoID: repo.ID,
		CommitID: commitID,
	}
	hasDBInfo, err := db.GetEngine(ctx).Get(addonDBInfo)
	if err != nil {
		return nil, err
	}
	if !hasDBInfo {
		return nil, errors.New("No database information for latest commit of add-on repository \"" + repo.Name + "\".")
	}

	// Parse the "info" file from the default branch
	var info api.AddonRepositoryInfo
	err_ := json.Unmarshal([]byte(addonDBInfo.InfoFile), &info)
	if err_ != nil {
		return nil, err_
	}

	// Get type from topics, if available
	var addonType = "worldmap" // Default type
	for _, topic := range repo.Topics {
		if topic == "world" || topic == "levelset" ||
				topic == "languagepack" || topic == "resourcepack" || topic == "addon" {
			addonType = topic
			break
		}
	}

	// List all screenshots
	screenshots := strings.Split(addonDBInfo.Screenshots, "/")

	// Return API add-on repository as a result
	return &api.AddonRepository{
		ID: repo.Name,
		Version: commitID,
		Type: addonType,
		Title: info.Title,
		Description: repo.Description,
		Author: repo.OwnerName,
		License: info.License,
		URL: repo.HTMLURL() + "/archive/" + commitID + ".zip",
		MD5: addonDBInfo.Md5,
		Screenshots: &api.AddonRepositoryScreenshots{
			BaseURL: repo.HTMLURL() + "/raw/commit/" + commitID + "/screenshots/",
			Files: screenshots,
		},
		Dependencies: info.Dependencies,
	}, nil
}

func innerToRepo(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode, isParent bool) *api.Repository {
	var parent *api.Repository

	cloneLink := repo.CloneLink()
	permission := &api.Permission{
		Admin: mode >= perm.AccessModeAdmin,
		Push:  mode >= perm.AccessModeWrite,
		Pull:  mode >= perm.AccessModeRead,
	}
	if !isParent {
		err := repo.GetBaseRepo(ctx)
		if err != nil {
			return nil
		}
		if repo.BaseRepo != nil {
			parent = innerToRepo(ctx, repo.BaseRepo, mode, true)
		}
	}

	// check enabled/disabled units
	hasIssues := false
	var externalTracker *api.ExternalTracker
	var internalTracker *api.InternalTracker
	if unit, err := repo.GetUnit(ctx, unit_model.TypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.GetUnit(ctx, unit_model.TypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:           config.ExternalTrackerURL,
			ExternalTrackerFormat:        config.ExternalTrackerFormat,
			ExternalTrackerStyle:         config.ExternalTrackerStyle,
			ExternalTrackerRegexpPattern: config.ExternalTrackerRegexpPattern,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.GetUnit(ctx, unit_model.TypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.GetUnit(ctx, unit_model.TypeExternalWiki); err == nil {
		hasWiki = true
		config := unit.ExternalWikiConfig()
		externalWiki = &api.ExternalWiki{
			ExternalWikiURL: config.ExternalWikiURL,
		}
	}
	hasPullRequests := false
	ignoreWhitespaceConflicts := false
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquash := false
	allowRebaseUpdate := false
	defaultDeleteBranchAfterMerge := false
	defaultMergeStyle := repo_model.MergeStyleMerge
	defaultAllowMaintainerEdit := false
	if unit, err := repo.GetUnit(ctx, unit_model.TypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
		allowRebaseUpdate = config.AllowRebaseUpdate
		defaultDeleteBranchAfterMerge = config.DefaultDeleteBranchAfterMerge
		defaultMergeStyle = config.GetDefaultMergeStyle()
		defaultAllowMaintainerEdit = config.DefaultAllowMaintainerEdit
	}
	hasProjects := false
	if _, err := repo.GetUnit(ctx, unit_model.TypeProjects); err == nil {
		hasProjects = true
	}

	hasReleases := false
	if _, err := repo.GetUnit(ctx, unit_model.TypeReleases); err == nil {
		hasReleases = true
	}

	hasPackages := false
	if _, err := repo.GetUnit(ctx, unit_model.TypePackages); err == nil {
		hasPackages = true
	}

	hasActions := false
	if _, err := repo.GetUnit(ctx, unit_model.TypeActions); err == nil {
		hasActions = true
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return nil
	}

	numReleases, _ := repo_model.GetReleaseCountByRepoID(ctx, repo.ID, repo_model.FindReleasesOptions{IncludeDrafts: false, IncludeTags: false})

	mirrorInterval := ""
	var mirrorUpdated time.Time
	if repo.IsMirror {
		var err error
		repo.Mirror, err = repo_model.GetMirrorByRepoID(ctx, repo.ID)
		if err == nil {
			mirrorInterval = repo.Mirror.Interval.String()
			mirrorUpdated = repo.Mirror.UpdatedUnix.AsTime()
		}
	}

	var transfer *api.RepoTransfer
	if repo.Status == repo_model.RepositoryPendingTransfer {
		t, err := models.GetPendingRepositoryTransfer(ctx, repo)
		if err != nil && !models.IsErrNoPendingTransfer(err) {
			log.Warn("GetPendingRepositoryTransfer: %v", err)
		} else {
			if err := t.LoadAttributes(ctx); err != nil {
				log.Warn("LoadAttributes of RepoTransfer: %v", err)
			} else {
				transfer = ToRepoTransfer(ctx, t)
			}
		}
	}

	var language string
	if repo.PrimaryLanguage != nil {
		language = repo.PrimaryLanguage.Language
	}

	repoAPIURL := repo.APIURL()

	return &api.Repository{
		ID:                            repo.ID,
		Owner:                         ToUserWithAccessMode(ctx, repo.Owner, mode),
		Name:                          repo.Name,
		FullName:                      repo.FullName(),
		Description:                   repo.Description,
		Private:                       repo.IsPrivate,
		Template:                      repo.IsTemplate,
		Empty:                         repo.IsEmpty,
		Archived:                      repo.IsArchived,
		Size:                          int(repo.Size / 1024),
		Fork:                          repo.IsFork,
		Parent:                        parent,
		Mirror:                        repo.IsMirror,
		HTMLURL:                       repo.HTMLURL(),
		SSHURL:                        cloneLink.SSH,
		CloneURL:                      cloneLink.HTTPS,
		OriginalURL:                   repo.SanitizedOriginalURL(),
		Website:                       repo.Website,
		Language:                      language,
		LanguagesURL:                  repoAPIURL + "/languages",
		Stars:                         repo.NumStars,
		Forks:                         repo.NumForks,
		Watchers:                      repo.NumWatches,
		OpenIssues:                    repo.NumOpenIssues,
		OpenPulls:                     repo.NumOpenPulls,
		Releases:                      int(numReleases),
		DefaultBranch:                 repo.DefaultBranch,
		Created:                       repo.CreatedUnix.AsTime(),
		Updated:                       repo.UpdatedUnix.AsTime(),
		Permissions:                   permission,
		HasIssues:                     hasIssues,
		ExternalTracker:               externalTracker,
		InternalTracker:               internalTracker,
		HasWiki:                       hasWiki,
		HasProjects:                   hasProjects,
		HasReleases:                   hasReleases,
		HasPackages:                   hasPackages,
		HasActions:                    hasActions,
		ExternalWiki:                  externalWiki,
		HasPullRequests:               hasPullRequests,
		IgnoreWhitespaceConflicts:     ignoreWhitespaceConflicts,
		AllowMerge:                    allowMerge,
		AllowRebase:                   allowRebase,
		AllowRebaseMerge:              allowRebaseMerge,
		AllowSquash:                   allowSquash,
		AllowRebaseUpdate:             allowRebaseUpdate,
		DefaultDeleteBranchAfterMerge: defaultDeleteBranchAfterMerge,
		DefaultMergeStyle:             string(defaultMergeStyle),
		DefaultAllowMaintainerEdit:    defaultAllowMaintainerEdit,
		AvatarURL:                     repo.AvatarLink(ctx),
		Internal:                      !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
		MirrorInterval:                mirrorInterval,
		MirrorUpdated:                 mirrorUpdated,
		RepoTransfer:                  transfer,
	}
}

// ToRepoTransfer convert a models.RepoTransfer to a structs.RepeTransfer
func ToRepoTransfer(ctx context.Context, t *models.RepoTransfer) *api.RepoTransfer {
	teams, _ := ToTeams(ctx, t.Teams, false)

	return &api.RepoTransfer{
		Doer:      ToUser(ctx, t.Doer, nil),
		Recipient: ToUser(ctx, t.Recipient, nil),
		Teams:     teams,
	}
}
