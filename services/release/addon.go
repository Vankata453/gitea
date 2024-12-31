// Copyright 2023 Vankata453
// SPDX-License-Identifier: MIT

package release

import (
	"io"
	"os"
	"context"
	"errors"
	"strings"
	"crypto/md5"
	"encoding/hex"
	"encoding/base64"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	addon_repo_model "code.gitea.io/gitea/models/repo_addon"
	activities_model "code.gitea.io/gitea/models/activities"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/timeutil"
	files_service "code.gitea.io/gitea/services/repository/files"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
)

func VerifyAddonRelease(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, rel *repo_model.Release) error {
	// Only Gitea admins can verify add-on releases
	if (!doer.IsAdmin) {
		return errors.New("Only admins can verify add-on releases.")
	}

	// Attempt to load saved data for the add-on repository from the database
	addonDBInfo := &addon_repo_model.AddonRepository{
		RepoID: repo.ID,
	}
	hasDBInfo, err := db.GetEngine(ctx).Get(addonDBInfo)
	if err != nil {
		return err
	}
	if hasDBInfo && addonDBInfo.ReleaseID == rel.ID {
		return nil // There is nothing to update.
	}

	// PROCEED WITH REGENERATING DATA
	addonDBInfo.ReleaseID = rel.ID

	// Open the repository
	gitRepo, err := git.OpenRepository(git.DefaultContext, repo_model.RepoPath(repo.OwnerName, repo.Name))
	if err != nil {
		return err
	}
	// Close the repository
	defer gitRepo.Close()

	// Make sure an archive of the latest commit is created, if data not available in the database
	archiveRequest, err := archiver_service.NewRequest(repo.ID, gitRepo, rel.Sha1, git.ZIP)
	if err != nil {
		return err
	}
	archiver, err := archiveRequest.Await(ctx)
	if err != nil {
		return err
	}

	// Get MD5 checksum of the archive
	archiveFile, err := os.Open("data/repo-archive/" + archiver.RelativePath())
	if err != nil {
		return err
	}
	defer archiveFile.Close()

	md5Hash := md5.New()
	_, err = io.Copy(md5Hash, archiveFile)
	if err != nil {
		return err
	}
	addonDBInfo.Md5 = hex.EncodeToString(md5Hash.Sum(nil)[:])

	// Get the "info" file from the default branch
	commit, err := gitRepo.GetCommit(rel.Sha1)
	if err != nil {
		return err
	}
	fileResponse, err := files_service.GetFileResponseFromCommit(ctx, repo, commit, rel.TagName, "info")
	if err != nil {
		return err
	}
	if fileResponse.Content == nil {
		return errors.New("Repository has no 'info' file!");
	}
	infoContent, err := base64.StdEncoding.DecodeString(*fileResponse.Content.Content)
	if err != nil {
		return err
	}
	addonDBInfo.InfoFile = string(infoContent)

	// Get all screenshot files from the Git tree
	var screenshots []string
	repoTree, err := gitRepo.GetTree(rel.Sha1)
	if err != nil || repoTree == nil {
		return err
	}
	entries, err := repoTree.ListEntriesRecursiveWithSize()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		entryName := entry.Name()
		if strings.HasPrefix(entryName, "screenshots/") {
			scrName := strings.TrimPrefix(entryName, "screenshots/")
			// Make sure the file has an extension and is not found in a sub-directory.
			if !strings.Contains(scrName, ".") || strings.Contains(scrName, "/")  {
				continue
			}
			screenshots = append(screenshots, scrName)
		}
	}
	addonDBInfo.Screenshots = strings.Join(screenshots, "/")

	// Insert new add-on data entry into the table
	if hasDBInfo {
		_, err = db.GetEngine(ctx).ID(addonDBInfo.ID).AllCols().Update(addonDBInfo)
		if err != nil {
			return errors.New("Cannot update database entry for add-on repository \"" + repo.Name + "\": " + err.Error())
		}
	} else {
		_, err = db.GetEngine(ctx).Insert(addonDBInfo)
		if err != nil {
			return errors.New("Cannot insert database entry for add-on repository \"" + repo.Name + "\": " + err.Error())
		}
	}

	// Set release to verified, insert into database
	rel.IsVerified = true
	rel.IsRejected = false
	rel.RejectionReason = ""
	rel.ReviewedUnix = timeutil.TimeStampNow()
	_, err = db.GetEngine(ctx).ID(rel.ID).Cols("is_verified", "is_rejected", "rejection_reason", "reviewed_unix").Update(rel)
	if err != nil {
		return errors.New("Cannot update database entry for release with tag \"" + rel.TagName + "\": " + err.Error())
	}

	err = rel.LoadAttributes(ctx)
	if err != nil {
		return errors.New("Error loading release attributes: " + err.Error())
	}
	err = activities_model.CreateReleaseReviewNotification(ctx, rel)
	if err != nil {
		return errors.New("Error pushing release review notification to repository owner: " + err.Error())
	}

	return nil
}

func RejectAddonRelease(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, rel *repo_model.Release, reason string) error {
	// Only Gitea admins can reject add-on releases
	if (!doer.IsAdmin) {
		return errors.New("Only admins can reject add-on releases.")
	}

	// Set release to rejected, set rejection reason, insert into database
	rel.IsVerified = false
	rel.IsRejected = true
	rel.RejectionReason = reason
	rel.ReviewedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(ctx).ID(rel.ID).Cols("is_verified", "is_rejected", "rejection_reason", "reviewed_unix").Update(rel)
	if err != nil {
		return errors.New("Cannot update database entry for release with tag \"" + rel.TagName + "\": " + err.Error())
	}

	err = rel.LoadAttributes(ctx)
	if err != nil {
		return errors.New("Error loading release attributes: " + err.Error())
	}
	err = activities_model.CreateReleaseReviewNotification(ctx, rel)
	if err != nil {
		return errors.New("Error pushing release review notification to repository owner: " + err.Error())
	}

	return nil
}
