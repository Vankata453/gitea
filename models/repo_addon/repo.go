// Copyright 2023 Vankata453
// SPDX-License-Identifier: MIT

package repo_addon

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
	"code.gitea.io/gitea/modules/git"
	files_service "code.gitea.io/gitea/services/repository/files"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
)

// AddonRepository represents saved data for add-on repositories
type AddonRepository struct {
	ID          int64  `xorm:"pk autoincr"`
	RepoID      int64  `xorm:"index unique(s)"`
	CommitID    string `xorm:"VARCHAR(40) unique(s)"`
	InfoFile    string `xorm:"TEXT JSON"`
	Md5         string `xorm:"TEXT"`
	Screenshots string `xorm:"TEXT"`
}

func RegenerateAddonRepositoryData(ctx context.Context, repo *repo_model.Repository) error {
	// Open the repository
	gitRepo, err := git.OpenRepository(git.DefaultContext, repo_model.RepoPath(repo.OwnerName, repo.Name))
	if err != nil {
		return err
	}
	// Close the repository
	defer gitRepo.Close()

	// Get the default branch
	branch, err := gitRepo.GetBranch(repo.DefaultBranch)
	if err != nil {
		return err
	}

	// Get last commit on the repository's default branch
	commit, err := branch.GetCommit()
	if err != nil {
		return err
	}
	commitID := hex.EncodeToString(commit.ID[:])

	// Attempt to load saved data for the add-on repository from the database
	addonDBInfo := &AddonRepository{
		RepoID: repo.ID,
		CommitID: commitID,
	}
	hasDBInfo, err := db.GetEngine(ctx).Get(addonDBInfo)
	if err != nil {
		return err
	}
	if hasDBInfo {
		return nil // There is nothing to update.
	}

	// Make sure an archive of the latest commit is created, if data not available in the database
	archiveRequest, err := archiver_service.NewRequest(repo.ID, gitRepo, commitID + ".zip")
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
	infoContentResponse, err := files_service.GetContents(ctx, repo, "info", repo.DefaultBranch, false)
	if err != nil {
		return err
	}
	infoContent, err := base64.StdEncoding.DecodeString(*infoContentResponse.Content)
	if err != nil {
		return err
	}
	addonDBInfo.InfoFile = string(infoContent)

	// Get all screenshot files from the Git tree
	var screenshots []string
	repoTree, err := gitRepo.GetTree(commitID)
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
	_, err = db.GetEngine(ctx).Insert(addonDBInfo)
	if err != nil {
		return errors.New("Cannot insert database entry for add-on repository \"" + repo.Name + "\": " + err.Error())
	}

	return nil
}
