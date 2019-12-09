package version

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver"
	"gopkg.in/src-d/go-git.v4"
)

type Method uint

const (
	GitBinary = iota
	GoGit
)

func Latest(method Method, path string) (string, error) {
	switch method {
	case GitBinary:
		return latestTag_gitbinary(path)
	case GoGit:
		return latestTag_gogit(path)
	default:
		return "", fmt.Errorf("invalid method: %d", method)
	}
}

func latestTag_gitbinary(path string) (string, error) {
	_, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("find git binary: %v", err)
	}

	revParse := exec.Command("git", "rev-parse", "HEAD")
	revParse.Dir = path
	revParseOut, err := revParse.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmd git rev-parse: %v", err)
	}
	headRev := strings.TrimSpace(string(revParseOut))

	showRef := exec.Command("git", "show-ref", "--tags", "--dereference")
	showRef.Dir = path
	showRefOut, err := showRef.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmd git show-ref: %v", err)
	}
	refs := strings.Split(string(showRefOut), "\n")

	var tags []struct{ Name, Revision string }
	for _, ref := range refs {
		if !strings.HasSuffix(ref, "^{}") { // not dereferenced
			continue
		}

		elems := strings.Split(ref, " ")
		if len(elems) < 2 {
			continue
		}

		revision := elems[0]
		name := strings.TrimSuffix(strings.TrimPrefix(elems[1], "refs/tags/"), "^{}")
		tags = append(tags, struct {
			Name     string
			Revision string
		}{
			Name:     name,
			Revision: revision,
		})
	}

	var ver *semver.Version
	for _, tag := range tags {
		v, err := semver.NewVersion(tag.Name)
		if err != nil {
			// FIXME: do debug logging
			continue
		}

		mergeBase := exec.Command("git", "merge-base", "--is-ancestor", tag.Revision, headRev)
		mergeBase.Dir = path
		err = mergeBase.Run()
		if err != nil {
			// FIXME: do debug logging
			continue
		}

		if ver == nil || v.GreaterThan(ver) {
			ver = v
		}
	}

	if ver != nil {
		return ver.String(), nil
	}

	return "", fmt.Errorf("no semver compliant tags found")
}

func latestTag_gogit(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("open git repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("get HEAD ref: %v", err)
	}
	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", fmt.Errorf("get HEAD commit: %v", err)
	}

	tagIter, err := repo.TagObjects()
	if err != nil {
		return "", fmt.Errorf("get tag objects: %w", err)
	}

	var ver *semver.Version
	for tag, err := tagIter.Next(); err == nil; tag, err = tagIter.Next() {
		v, err := semver.NewVersion(tag.Name)
		if err != nil {
			// FIXME: do debug logging
			continue
		}

		tagCommit, err := repo.CommitObject(tag.Target)
		if err != nil {
			// FIXME: do debug logging
			continue
		}

		isAncestor, err := tagCommit.IsAncestor(headCommit)
		if err != nil || !isAncestor {
			// FIXME: do debug logging
			continue
		}

		if ver == nil || v.GreaterThan(ver) { // Masterminds/semver really ought to support using the nil value in Version.Compare
			ver = v
		}
	}
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("get next tag: %w", err)
	}

	if ver == nil {
		return "", fmt.Errorf("no parseable tag found")
	}

	return ver.String(), nil
}
