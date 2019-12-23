// Copyright 2016 The Kubernetes Authors All rights reserved.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/kubernetes/helm/blob/master/LICENSE

package helm

import (
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/vinkdong/gox/log"
	"k8s.io/helm/pkg/manifest"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"k8s.io/client-go/discovery"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/hooks"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	util "k8s.io/helm/pkg/releaseutil"
	"k8s.io/helm/pkg/repo"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/version"
)

type result struct {
	hooks   []*release.Hook
	generic []tiller.Manifest
}

type manifestFile struct {
	entries map[string]string
	path    string
	apis    chartutil.VersionSet
}

type kindSorter struct {
	ordering  map[string]int
	manifests []tiller.Manifest
}

func getChart(
	repoURL string,
	chartName string,
	chartVersion string) (*chart.Chart, error) {
	dl := downloader.ChartDownloader{
		HelmHome: settings.Home,
		Out:      os.Stdout,
		Getters:  getter.All(settings),
		Verify:   downloader.VerifyNever,
	}

	certFile := ""
	keyFile := ""
	caFile := ""
	chartURL, err := repo.FindChartInRepoURL(
		repoURL,
		chartName,
		chartVersion,
		certFile, keyFile, caFile, getter.All(settings))
	if err != nil {
		return nil, fmt.Errorf("find chart: %v", err)
	}

	glog.V(1).Infof("Downloading %s ...", chartURL)

	fname, _, err := dl.DownloadTo(chartURL, chartVersion, settings.Home.Archive())
	if err != nil {
		return nil, fmt.Errorf("download chart: %v", err)
	}
	glog.V(1).Infof("Downloaded %s to %s", chartURL, fname)

	return chartutil.LoadFile(fname)
}

func capabilities(disc discovery.DiscoveryInterface) (*chartutil.Capabilities, error) {
	sv, err := disc.ServerVersion()
	if err != nil {
		return nil, err
	}
	vs, err := tiller.GetVersionSet(disc)
	if err != nil {
		return nil, fmt.Errorf("Could not get apiVersions from Kubernetes: %s", err)
	}
	return &chartutil.Capabilities{
		APIVersions:   vs,
		KubeVersion:   sv,
		TillerVersion: version.GetVersionProto(),
	}, nil
}

// sortManifests takes a map of filename/YAML contents, splits the file
// by manifest entries, and sorts the entries into hook types.
//
// The resulting hooks struct will be populated with all of the generated hooks.
// Any file that does not declare one of the hook types will be placed in the
// 'generic' bucket.
//
// Files that do not parse into the expected format are simply placed into a map and
// returned.
func sortManifests(files map[string]string, apis chartutil.VersionSet, sort tiller.SortOrder) ([]*release.Hook, []tiller.Manifest, error) {
	result := &result{}

	for filePath, c := range files {

		// Skip partials. We could return these as a separate map, but there doesn't
		// seem to be any need for that at this time.
		if strings.HasPrefix(path.Base(filePath), "_") {
			continue
		}
		// Skip empty files and log this.
		if len(strings.TrimSpace(c)) == 0 {
			glog.Infof("info: manifest %q is empty. Skipping.", filePath)
			continue
		}

		manifestFile := &manifestFile{
			entries: util.SplitManifests(c),
			path:    filePath,
			apis:    apis,
		}

		if err := manifestFile.sort(result); err != nil {
			return result.hooks, result.generic, err
		}
	}

	return result.hooks, sortByKind(result.generic, sort), nil
}

func sortByKind(manifests []tiller.Manifest, ordering tiller.SortOrder) []tiller.Manifest {
	ks := newKindSorter(manifests, ordering)
	sort.Sort(ks)
	return ks.manifests
}

func (k *kindSorter) Len() int { return len(k.manifests) }

func (k *kindSorter) Swap(i, j int) { k.manifests[i], k.manifests[j] = k.manifests[j], k.manifests[i] }

func (k *kindSorter) Less(i, j int) bool {
	a := k.manifests[i]
	b := k.manifests[j]
	first, aok := k.ordering[a.Head.Kind]
	second, bok := k.ordering[b.Head.Kind]
	// if same kind (including unknown) sub sort alphanumeric
	if first == second {
		// if both are unknown and of different kind sort by kind alphabetically
		if !aok && !bok && a.Head.Kind != b.Head.Kind {
			return a.Head.Kind < b.Head.Kind
		}
		return a.Name < b.Name
	}
	// unknown kind is last
	if !aok {
		return false
	}
	if !bok {
		return true
	}
	// sort different kinds
	return first < second
}

func newKindSorter(m []tiller.Manifest, s tiller.SortOrder) *kindSorter {
	o := make(map[string]int, len(s))
	for v, k := range s {
		o[k] = v
	}

	return &kindSorter{
		manifests: m,
		ordering:  o,
	}
}

type Manifest = manifest.Manifest

func (file *manifestFile) sort(result *result) error {
	for _, m := range file.entries {
		var entry util.SimpleHead
		err := yaml.Unmarshal([]byte(m), &entry)

		if err != nil {
			e := fmt.Errorf("YAML parse error on %s: %s", file.path, err)
			return e
		}

		if !hasAnyAnnotation(entry) {
			result.generic = append(result.generic, Manifest{
				Name:    file.path,
				Content: m,
				Head:    &entry,
			})
			continue
		}

		hookTypes, ok := entry.Metadata.Annotations[hooks.HookAnno]
		if !ok {
			result.generic = append(result.generic, Manifest{
				Name:    file.path,
				Content: m,
				Head:    &entry,
			})
			continue
		}

		hw := calculateHookWeight(entry)

		h := &release.Hook{
			Name:           entry.Metadata.Name,
			Kind:           entry.Kind,
			Path:           file.path,
			Manifest:       m,
			Events:         []release.Hook_Event{},
			Weight:         hw,
			DeletePolicies: []release.Hook_DeletePolicy{},
		}

		isUnknownHook := false
		for _, hookType := range strings.Split(hookTypes, ",") {
			hookType = strings.ToLower(strings.TrimSpace(hookType))
			e, ok := events[hookType]
			if !ok {
				isUnknownHook = true
				break
			}
			h.Events = append(h.Events, e)
		}

		if isUnknownHook {
			log.Infof("info: skipping unknown hook: %q", hookTypes)
			continue
		}

		result.hooks = append(result.hooks, h)

		operateAnnotationValues(entry, hooks.HookDeleteAnno, func(value string) {
			policy, exist := deletePolices[value]
			if exist {
				h.DeletePolicies = append(h.DeletePolicies, policy)
			} else {
				log.Infof("info: skipping unknown hook delete policy: %q", value)
			}
		})

		// Only check for delete timeout annotation if there is a deletion policy.
		//if len(h.DeletePolicies) > 0 {
		//	//h.DeleteTimeout = defaultHookDeleteTimeoutInSeconds
		//	operateAnnotationValues(entry, hooks.HookDeleteTimeoutAnno, func(value string) {
		//		timeout, err := strconv.ParseInt(value, 10, 64)
		//		if err != nil || timeout < 0 {
		//			log.Infof("info: ignoring invalid hook delete timeout value: %q", value)
		//		}
		//		//h.DeleteTimeout = timeout
		//	})
		//}
	}

	return nil
}

func hasAnyAnnotation(entry util.SimpleHead) bool {
	if entry.Metadata == nil ||
		entry.Metadata.Annotations == nil ||
		len(entry.Metadata.Annotations) == 0 {
		return false
	}

	return true
}

func calculateHookWeight(entry util.SimpleHead) int32 {
	hws := entry.Metadata.Annotations[hooks.HookWeightAnno]
	hw, err := strconv.Atoi(hws)
	if err != nil {
		hw = 0
	}

	return int32(hw)
}

func operateAnnotationValues(entry util.SimpleHead, annotation string, operate func(p string)) {
	if dps, ok := entry.Metadata.Annotations[annotation]; ok {
		for _, dp := range strings.Split(dps, ",") {
			dp = strings.ToLower(strings.TrimSpace(dp))
			operate(dp)
		}
	}
}
