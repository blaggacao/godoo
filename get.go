// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO: Dashboard upload

package main

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var cmdGet = &Command{
	UsageLine: "get [-a] [-d] [-fix] [-n] [-p n] [-u] [-v] [-x] [packages]",
	Short:     "download and install packages and dependencies",
	Long: `
Get downloads and installs the packages named by the import paths,
along with their dependencies.

The -a, -n, -v, -x, and -p flags have the same meaning as in 'go build'
and 'go install'.  See 'go help build'.

The -d flag instructs get to stop after downloading the packages; that is,
it instructs get not to install the packages.

The -fix flag instructs get to run the fix tool on the downloaded packages
before resolving dependencies or building the code.

The -u flag instructs get to use the network to update the named packages
and their dependencies.  By default, get uses the network to check out
missing packages but does not use it to look for updates to existing packages.

When checking out or updating a package, get looks for a branch or
tag that matches the locally installed version of Go. If the local
version "is release.rNN", it searches for "go.rNN". (For an
installation using Go version "weekly.YYYY-MM-DD", it searches for a
package version labeled "go.YYYY-MM-DD".)  If the desired version
cannot be found but others exist with labels in the correct format,
get retrieves the most recent version before the desired label.
Finally, if all else fails it retrieves the most recent version of
the package.

For more about specifying packages, see 'go help packages'.

For more about how 'go get' finds source code to
download, see 'go help remote'.

See also: go build, go install, go clean.
	`,
}

var getS = cmdGet.Flag.Bool("s", false, "")
var getU = cmdGet.Flag.Bool("u", false, "")

func init() {
	cmdGet.Run = runGet // break init loop
}

func runGet(cmd *Command, args []string) {
	// Phase 1.  Download/update.
	var stk importStack
	for _, arg := range downloadPaths(args) {
		download(arg, &stk)
	}
	exitIfErrors()

	// Phase 2. Rescan packages and reevaluate args list.

	// Code we downloaded and all code that depends on it
	// needs to be evicted from the package cache so that
	// the information will be recomputed.  Instead of keeping
	// track of the reverse dependency information, evict
	// everything.
	for name := range packageCache {
		delete(packageCache, name)
	}

	args = importPaths(args)

}

// downloadPath prepares the list of paths to pass to download.
// It expands ... patterns that can be expanded.  If there is no match
// for a particular pattern, downloadPaths leaves it in the result list,
// in the hope that we can figure out the repository from the
// initial ...-free prefix.
func downloadPaths(args []string) []string {
	args = importPathsNoDotExpansion(args) // OK
	var out []string
	for _, a := range args {
		if strings.Contains(a, "...") {
			var expand []string
			// Use matchPackagesInFS to avoid printing
			// warnings.  They will be printed by the
			// eventual call to importPaths instead.
			if build.IsLocalImport(a) { // TODO: Call to build to determine if it is a local import ref.
				expand = matchPackagesInFS(a) // OK
			} else {
				expand = matchPackages(a) // OK
			}
			if len(expand) > 0 {
				out = append(out, expand...)
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

// downloadCache records the import paths we have already
// considered during the download, to avoid duplicate work when
// there is more than one dependency sequence leading to
// a particular package.
var downloadCache = map[string]bool{}

// downloadRootCache records the version control repository
// root directories we have already considered during the download.
// For example, all the packages in the code.google.com/p/codesearch repo
// share the same root (the directory for that path), and we only need
// to run the hg commands to consider each repository once.
var downloadRootCache = map[string]bool{}

// download runs the download half of the get command
// for the package named by the argument.
func download(arg string, stk *importStack) {
	p := loadPackage(arg, stk) // TODO Various calls to pkg.go

	// There's nothing to do if this is a package in the standard library.
	if p.Standard { // NOTE corresponds to odoo standard addons.
		return
	}

	// Only process each package once.
	if downloadCache[arg] {
		return
	}
	downloadCache[arg] = true

	pkgs := []*Package{p}
	wildcardOkay := len(*stk) == 0

	// Download if the package is missing, or update if we're using -u.
	if p.Dir == "" || *getU {
		// The actual download.
		stk.push(p.ImportPath)
		err := downloadPackage(p)
		if err != nil {
			errorf("%s", &PackageError{ImportStack: stk.copy(), Err: err.Error()})
			stk.pop()
			return
		}

		args := []string{arg}
		// If the argument has a wildcard in it, re-evaluate the wildcard.
		// We delay this until after reloadPackage so that the old entry
		// for p has been replaced in the package cache.
		if wildcardOkay && strings.Contains(arg, "...") {
			if build.IsLocalImport(arg) { //TODO Same call to build.
				args = matchPackagesInFS(arg)
			} else {
				args = matchPackages(arg)
			}
		}

		// Clear all relevant package cache entries before
		// doing any new loads.
		for _, arg := range args {
			p := packageCache[arg]
			if p != nil {
				delete(packageCache, p.Dir)
				delete(packageCache, p.ImportPath)
			}
		}

		pkgs = pkgs[:0]
		for _, arg := range args {
			stk.push(arg)
			p := loadPackage(arg, stk) // TODO Various calls to pkg.go
			stk.pop()
			if p.Error != nil {
				errorf("%s", p.Error)
				continue
			}
			pkgs = append(pkgs, p)
		}
	}

	// Process package, which might now be multiple packages
	// due to wildcard expansion.
	for _, p := range pkgs {
		// Process dependencies, now that we know what they are.
		for _, dep := range p.deps {
			download(dep.ImportPath, stk)
		}
	}
}

// downloadPackage runs the create or download command
// to make the first copy of or update a copy of the given package.
func downloadPackage(p *Package) error {
	var (
		vcs            *vcsCmd
		repo, rootPath string
		err            error
	)
	if p.build.SrcRoot != "" { // TODO unsure: p.build
		// Directory exists.  Look for checkout along path to src.
		vcs, rootPath, err = vcsForDir(p)
		if err != nil {
			return err
		}
		repo = "<local>" // should be unused; make distinctive
	} else {
		// Analyze the import path to determine the version control system,
		// repository, and the import path for the root of the repository.
		rr, err := repoRootForImportPath(p.ImportPath)
		if err != nil {
			return err
		}
		vcs, repo, rootPath = rr.vcs, rr.repo, rr.root
	}

	if p.build.SrcRoot == "" {
		// Package not found.  Put in first directory of $GOPATH or else $GOROOT.
		// Guard against people setting GOPATH=$GOROOT.  We have to use
		// $GOROOT's directory hierarchy (src/pkg, not just src) in that case.
		if list := filepath.SplitList(buildContext.GOPATH); len(list) > 0 && list[0] != goroot { // TODO Reference to GoPath and GoRoot
			p.build.SrcRoot = list[0]
		} else {
			p.build.SrcRoot = filepath.Join(goroot, "oPath")
		}
	}
	root := filepath.Join(p.build.SrcRoot, rootPath)
	// If we've considered this repository already, don't do it again.
	if downloadRootCache[root] {
		return nil
	}
	downloadRootCache[root] = true

	if buildV {
		fmt.Fprintf(os.Stderr, "%s (download)\n", rootPath)
	}

	// Check that this is an appropriate place for the repo to be checked out.
	// The target directory must either not exist or have a repo checked out already.
	meta := filepath.Join(root, "."+vcs.cmd)
	st, err := os.Stat(meta)
	if err == nil && !st.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", meta)
	}
	if err != nil {
		// Metadata directory does not exist.  Prepare to checkout new copy.
		// Some version control tools require the target directory not to exist.
		// We require that too, just to avoid stepping on existing work.
		if _, err := os.Stat(root); err == nil {
			return fmt.Errorf("%s exists but %s does not - stale checkout?", root, meta)
		}
		// Some version control tools require the parent of the target to exist.
		parent, _ := filepath.Split(root)
		if err := os.MkdirAll(parent, 0777); err != nil {
			return err
		}
		if err = vcs.create(root, repo); err != nil {
			return err
		}
	} else {
		// Metadata directory does exist; download incremental updates.
		if err = vcs.download(root); err != nil {
			return err
		}
	}

	if buildN {
		// Do not show tag sync in -n; it's noise more than anything,
		// and since we're not running commands, no tag will be found.
		// But avoid printing nothing.
		fmt.Fprintf(os.Stderr, "# cd %s; %s sync/update\n", root, vcs.cmd)
		return nil
	}

	// Select and sync to appropriate version of the repository.
	tags, err := vcs.tags(root)
	if err != nil {
		return err
	}
	vers := "9.0" // TODO Get the current Odoo Version, formerly: runtime.Version()
	if i := strings.Index(vers, " "); i >= 0 {
		vers = vers[:i]
	}
	if err := vcs.tagSync(root, selectTag(vers, tags)); err != nil {
		return err
	}

	return nil
}

// goTag matches go release tags such as go1 and go1.2.3.
// The numbers involved must be small (at most 4 digits),
// have no unnecessary leading zeros, and the version cannot
// end in .0 - it is go1, not go1.0 or go1.0.0.
var goTag = regexp.MustCompile(
	`^((0|[1-9][0-9]{0,2})\.)*([0-9])$`, // NOTE match odoo tags
)

// selectTag returns the closest matching tag for a given version.
// Closest means the latest one that is not after the current release.
// Version "goX" (or "goX.Y" or "goX.Y.Z") matches tags of the same form.
// Version "release.rN" matches tags of the form "go.rN" (N being a floating-point number).
// Version "weekly.YYYY-MM-DD" matches tags like "go.weekly.YYYY-MM-DD".
func selectTag(goVersion string, tags []string) (match string) {
	const wMark = ".weekly."
	if strings.Contains(goVersion, wMark) {
		b := strings.Split(goVersion, wMark)[0] // "9.0"
		if goTag.MatchString(b) {
			p := strings.SplitAfter(goVersion, wMark)[0] // "9.0.weekly."
			v := strings.SplitAfter(goVersion, wMark)[1] // "2015-10-31"
			for _, t := range tags {
				if !strings.Contains(t, p) {
					continue
				}
				if match < t && t[len(p):] <= v {
					match = t
				}
			}
		}
	}

	if goTag.MatchString(goVersion) {
		v := goVersion
		for _, t := range tags {
			if !goTag.MatchString(t) {
				continue
			}
			if cmpGoVersion(match, t) < 0 && cmpGoVersion(t, v) <= 0 {
				match = t
			}
		}
	}

	return match
}

// cmpGoVersion returns -1, 0, +1 reporting whether
// x < y, x == y, or x > y.
func cmpGoVersion(x, y string) int {
	// Malformed strings compare less than well-formed strings.
	if !goTag.MatchString(x) {
		return -1
	}
	if !goTag.MatchString(y) {
		return +1
	}

	// Compare numbers in sequence.
	xx := strings.Split(x, ".")
	yy := strings.Split(y, ".")

	for i := 0; i < len(xx) && i < len(yy); i++ {
		// The Atoi are guaranteed to succeed
		// because the versions match goTag.
		xi, _ := strconv.Atoi(xx[i])
		yi, _ := strconv.Atoi(yy[i])
		if xi < yi {
			return -1
		} else if xi > yi {
			return +1
		}
	}

	if len(xx) < len(yy) {
		return -1
	}
	if len(xx) > len(yy) {
		return +1
	}
	return 0
}
