/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package upgrade

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/helm/helm-2to3/pkg/common"
	v2 "github.com/helm/helm-2to3/pkg/v2"
)

type CleanupOptions struct {
	ConfigCleanup    bool
	DryRun           bool
	ReleaseName      string
	ReleaseCleanup   bool
	StorageType      string
	TillerCleanup    bool
	TillerLabel      string
	TillerNamespace  string
	TillerOutCluster bool
}

func RunCleanup(releaseName string) error {
	cleanupOptions := CleanupOptions{
		ConfigCleanup:    false,
		DryRun:           false,
		ReleaseCleanup:   false,
		ReleaseName:      releaseName,
		StorageType:      settings.ReleaseStorage,
		TillerCleanup:    false,
		TillerLabel:      settings.Label,
		TillerNamespace:  settings.TillerNamespace,
		TillerOutCluster: settings.TillerOutCluster,
	}

	kubeConfig := common.KubeConfig{
		Context: settings.KubeContext,
		File:    settings.KubeConfigFile,
	}

	return cleanup(cleanupOptions, kubeConfig)
}

// Cleanup will delete all release data for in specified namespace and owner label. It will remove
// the Tiller server deployed as per namespace and owner label. It is also delete the Helm gv2 home directory
// which contains the Helm configuration. Helm v2 will be unusable after this operation.
func cleanup(cleanupOptions CleanupOptions, kubeConfig common.KubeConfig) error {
	var message strings.Builder

	if cleanupOptions.ReleaseName != "" {
		if cleanupOptions.ConfigCleanup || cleanupOptions.TillerCleanup {
			return errors.New("cleanup of a specific release is a singular operation. Other operations like configuration cleanup or Tiller cleanup are not allowed in conjunction with the operation")
		}
		cleanupOptions.ReleaseCleanup = true
	} else {
		return errors.New("release name can't be empty")
	}

	if cleanupOptions.DryRun {
		log.Println("NOTE: This is in dry-run mode, the following actions will not be executed.")
		log.Println("Run without --dry-run to take the actions described below:")
		log.Println()
	}

	fmt.Fprint(&message, "WARNING: ")
	if cleanupOptions.ReleaseCleanup {
		fmt.Fprint(&message, fmt.Sprintf("\"Release '%s' Data\" ", cleanupOptions.ReleaseName))
	}
	if cleanupOptions.TillerCleanup {
		fmt.Fprint(&message, "\"Tiller\" ")
	}
	fmt.Fprintln(&message, "will be removed. ")

	fmt.Println(message.String())

	log.Printf("\nHelm v2 data will be cleaned up.\n")

	var err error
	if cleanupOptions.ReleaseCleanup {
		log.Printf("[Helm 2] Release '%s' will be deleted.\n", cleanupOptions.ReleaseName)
		retrieveOptions := v2.RetrieveOptions{
			ReleaseName:      cleanupOptions.ReleaseName,
			TillerNamespace:  cleanupOptions.TillerNamespace,
			TillerLabel:      cleanupOptions.TillerLabel,
			TillerOutCluster: cleanupOptions.TillerOutCluster,
			StorageType:      cleanupOptions.StorageType,
		}
		// Get the releases versions as its the versions that are deleted
		v2Releases, err := v2.GetReleaseVersions(retrieveOptions, kubeConfig)
		if err != nil {
			return err
		}
		versions := []int32{}
		v2RelVerLen := len(v2Releases)
		for i := 0; i < v2RelVerLen; i++ {
			v2Release := v2Releases[i]
			versions = append(versions, v2Release.Version)
		}
		deleteOptions := v2.DeleteOptions{
			DryRun:   cleanupOptions.DryRun,
			Versions: versions,
		}
		err = v2.DeleteReleaseVersions(retrieveOptions, deleteOptions, kubeConfig)
		if err != nil {
			return err
		}
	}
	if !cleanupOptions.DryRun {
		log.Printf("[Helm 2] Release '%s' deleted.\n", cleanupOptions.ReleaseName)
	}

	if !cleanupOptions.TillerOutCluster && cleanupOptions.TillerCleanup {
		log.Printf("[Helm 2] Tiller in \"%s\" namespace will be removed.\n", cleanupOptions.TillerNamespace)
		err = v2.RemoveTiller(cleanupOptions.TillerNamespace, cleanupOptions.DryRun)
		if err != nil {
			return err
		}
		if !cleanupOptions.DryRun {
			log.Printf("[Helm 2] Tiller in \"%s\" namespace was removed.\n", cleanupOptions.TillerNamespace)
		}
	}

	if !cleanupOptions.DryRun {
		log.Println("Helm v2 data was cleaned up successfully.")
	}
	return nil
}
