// Copyright Dit 2026
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

var client *github.Client
var ctx = context.Background()

func init() {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_PAT")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client = github.NewClient(tc)
}

func main() {
	var linuxVariant string
	flag.StringVar(&linuxVariant, "l", "linuxVariant", "specify linux variant")
	flag.Parse()
	releases, _, err := client.Repositories.ListReleases(ctx, "actions", "runner-images", &github.ListOptions{PerPage: 100})
	if err != nil {
		return
	}
	kernels := extractKernels(releases, linuxVariant)
	kernels = dedupe(kernels)
	result, _ := json.Marshal(kernels)
	fmt.Println(string(result))
}

// extractKernelVersions extracts kernel version strings from a release body.
// It looks for lines containing "Linux kernel version:" or "Kernel Version:".
func extractKernelVersions(body string) []string {
	var versions []string
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "Linux kernel version:") {
			versions = append(versions, strings.TrimSpace(strings.TrimPrefix(line, "- Linux kernel version: ")))
		}
		if strings.Contains(line, "Kernel Version:") {
			versions = append(versions, strings.TrimSpace(strings.TrimPrefix(line, "- Kernel Version: ")))
		}
	}
	return versions
}

// extractKernels filters releases by linuxVariant and extracts kernel versions.
// If linuxVariant contains commas, each comma-separated value is checked independently.
func extractKernels(releases []*github.RepositoryRelease, linuxVariant string) []string {
	var kernels []string
	for _, release := range releases {
		if strings.Contains(linuxVariant, ",") {
			for _, v := range strings.Split(linuxVariant, ",") {
				if strings.Contains(release.GetTagName(), v) {
					kernels = append(kernels, extractKernelVersions(release.GetBody())...)
				}
			}
		} else {
			if strings.Contains(release.GetTagName(), linuxVariant) {
				kernels = append(kernels, extractKernelVersions(release.GetBody())...)
			}
		}
	}
	return kernels
}

// https://go.dev/play/p/iyb97KcftMa
func dedupe(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}
