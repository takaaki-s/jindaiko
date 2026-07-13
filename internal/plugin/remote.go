package plugin

// This file bridges the registry document (pkg/plugin/manifest.RegistryDocument)
// and the on-disk install flow. Given a plugin name and an optional version pin,
// it looks up the entry, picks a version, and constructs a Source whose CloneURL
// is the entry's repo and whose Ref is the version's SHA — so the existing
// Fetch flow can install a registry-listed plugin at a specific commit without
// knowing anything about registry semantics.

import (
	"fmt"
	"strings"

	"github.com/takaaki-s/jind-ai/pkg/plugin/manifest"
)

// RemoteResolution is the outcome of resolving a name/version pair against the
// registry: the matching entry and the exact version chosen. Both are handed
// to the consent screen so the user sees registry-side metadata (repo, jin
// compat range, unverified status) before the clone starts.
type RemoteResolution struct {
	Entry   manifest.RegistryEntry
	Version manifest.RegistryVersion
}

// Source returns the Source that, passed to Fetch, will clone the resolved
// repo at the resolved SHA. Raw records the "<repo>@<sha>" pair so the lock
// file (and any later update) can trace back to the origin. A registry entry
// normally records Repo in the "github.com/owner/name" shape; if the entry
// already carries a scheme (http/https/git/file/scp-style) it is preserved
// verbatim so mirrors, forks, and test fixtures can round-trip.
func (r RemoteResolution) Source() Source {
	cloneURL := repoToCloneURL(r.Entry.Repo)
	raw := cloneURL + "@" + r.Version.SHA
	return Source{Raw: raw, CloneURL: cloneURL, Ref: r.Version.SHA}
}

func repoToCloneURL(repo string) string {
	switch {
	case strings.Contains(repo, "://"), strings.HasPrefix(repo, "git@"):
		return repo
	default:
		return "https://" + repo
	}
}

// ResolveRemote looks up name in doc and picks a version. An empty versionPin
// selects the entry's LatestVersion (an error if the registry marked it
// orphaned without a latest). A concrete pin must match one of the entry's
// listed versions; unknown pins are rejected rather than silently downgraded
// to latest, so an out-of-registry tag does not turn into a surprise install.
func ResolveRemote(name, versionPin string, doc *manifest.RegistryDocument) (*RemoteResolution, error) {
	if doc == nil {
		return nil, fmt.Errorf("registry document is not loaded")
	}
	entry := doc.Find(name)
	if entry == nil {
		return nil, fmt.Errorf("plugin %q is not in the registry (try `jin plugin ls-remote` to browse)", name)
	}

	target := versionPin
	if target == "" {
		target = entry.LatestVersion
		if target == "" {
			return nil, fmt.Errorf("plugin %q has no latest_version in the registry", name)
		}
	}

	for _, v := range entry.Versions {
		if v.Version == target {
			if v.SHA == "" {
				return nil, fmt.Errorf("plugin %q version %s has no sha in the registry", name, target)
			}
			return &RemoteResolution{Entry: *entry, Version: v}, nil
		}
	}

	available := make([]string, 0, len(entry.Versions))
	for _, v := range entry.Versions {
		available = append(available, v.Version)
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("plugin %q has no versions in the registry", name)
	}
	return nil, fmt.Errorf("plugin %q version %s not in registry (available: %v)", name, target, available)
}
