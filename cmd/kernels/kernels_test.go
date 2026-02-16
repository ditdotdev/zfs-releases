package main

import (
	"testing"

	"github.com/google/go-github/v39/github"
)

func TestDedupeEmpty(t *testing.T) {
	result := dedupe([]string{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestDedupeNoDuplicates(t *testing.T) {
	input := []string{"a", "b", "c"}
	result := dedupe(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("expected [a b c], got %v", result)
	}
}

func TestDedupeWithDuplicates(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b", "a"}
	result := dedupe(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("expected [a b c], got %v", result)
	}
}

func TestDedupeAllIdentical(t *testing.T) {
	input := []string{"x", "x", "x"}
	result := dedupe(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	if result[0] != "x" {
		t.Errorf("expected [x], got %v", result)
	}
}

func TestDedupePreservesOrder(t *testing.T) {
	input := []string{"c", "a", "b", "a", "c"}
	result := dedupe(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	// Order should match first occurrence
	if result[0] != "c" || result[1] != "a" || result[2] != "b" {
		t.Errorf("expected [c a b], got %v", result)
	}
}

func TestDedupeNil(t *testing.T) {
	result := dedupe(nil)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestExtractKernelVersionsOlderFormat(t *testing.T) {
	body := "Some text\n- Linux kernel version: 5.15.0-1057-azure\nMore text"
	result := extractKernelVersions(body)
	if len(result) != 1 {
		t.Fatalf("expected 1 version, got %d", len(result))
	}
	if result[0] != "5.15.0-1057-azure" {
		t.Errorf("expected 5.15.0-1057-azure, got %s", result[0])
	}
}

func TestExtractKernelVersionsNewerFormat(t *testing.T) {
	body := "Some text\n- Kernel Version: 6.8.0-90-generic\nMore text"
	result := extractKernelVersions(body)
	if len(result) != 1 {
		t.Fatalf("expected 1 version, got %d", len(result))
	}
	if result[0] != "6.8.0-90-generic" {
		t.Errorf("expected 6.8.0-90-generic, got %s", result[0])
	}
}

func TestExtractKernelVersionsBothFormats(t *testing.T) {
	body := "- Linux kernel version: 5.15.0-1057-azure\n- Kernel Version: 6.8.0-90-generic\n"
	result := extractKernelVersions(body)
	if len(result) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(result))
	}
	if result[0] != "5.15.0-1057-azure" {
		t.Errorf("expected 5.15.0-1057-azure, got %s", result[0])
	}
	if result[1] != "6.8.0-90-generic" {
		t.Errorf("expected 6.8.0-90-generic, got %s", result[1])
	}
}

func TestExtractKernelVersionsNoMatch(t *testing.T) {
	body := "Some random text\nNo kernel info here\n"
	result := extractKernelVersions(body)
	if len(result) != 0 {
		t.Errorf("expected 0 versions, got %v", result)
	}
}

func TestExtractKernelVersionsEmptyBody(t *testing.T) {
	result := extractKernelVersions("")
	if len(result) != 0 {
		t.Errorf("expected 0 versions, got %v", result)
	}
}

func TestExtractKernelVersionsMultipleOlderFormat(t *testing.T) {
	body := "- Linux kernel version: 5.15.0-1057-azure\n- Linux kernel version: 5.15.0-1058-azure\n"
	result := extractKernelVersions(body)
	if len(result) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(result))
	}
}

func strPtr(s string) *string { return &s }

func TestExtractKernelsSingleVariant(t *testing.T) {
	releases := []*github.RepositoryRelease{
		{TagName: strPtr("ubuntu22/20240101"), Body: strPtr("- Kernel Version: 6.5.0-44-generic\n")},
		{TagName: strPtr("ubuntu24/20240101"), Body: strPtr("- Kernel Version: 6.8.0-90-generic\n")},
		{TagName: strPtr("windows2022/20240101"), Body: strPtr("No kernel info\n")},
	}

	result := extractKernels(releases, "ubuntu22")
	if len(result) != 1 {
		t.Fatalf("expected 1 kernel, got %d: %v", len(result), result)
	}
	if result[0] != "6.5.0-44-generic" {
		t.Errorf("expected 6.5.0-44-generic, got %s", result[0])
	}
}

func TestExtractKernelsMultipleVariants(t *testing.T) {
	releases := []*github.RepositoryRelease{
		{TagName: strPtr("ubuntu22/20240101"), Body: strPtr("- Kernel Version: 6.5.0-44-generic\n")},
		{TagName: strPtr("ubuntu24/20240101"), Body: strPtr("- Kernel Version: 6.8.0-90-generic\n")},
		{TagName: strPtr("windows2022/20240101"), Body: strPtr("No kernel info\n")},
	}

	result := extractKernels(releases, "ubuntu22,ubuntu24")
	if len(result) != 2 {
		t.Fatalf("expected 2 kernels, got %d: %v", len(result), result)
	}
	if result[0] != "6.5.0-44-generic" {
		t.Errorf("expected 6.5.0-44-generic, got %s", result[0])
	}
	if result[1] != "6.8.0-90-generic" {
		t.Errorf("expected 6.8.0-90-generic, got %s", result[1])
	}
}

func TestExtractKernelsNoMatchingVariant(t *testing.T) {
	releases := []*github.RepositoryRelease{
		{TagName: strPtr("windows2022/20240101"), Body: strPtr("- Kernel Version: 6.5.0\n")},
	}

	result := extractKernels(releases, "ubuntu22")
	if len(result) != 0 {
		t.Errorf("expected 0 kernels, got %v", result)
	}
}

func TestExtractKernelsEmptyReleases(t *testing.T) {
	result := extractKernels(nil, "ubuntu22")
	if len(result) != 0 {
		t.Errorf("expected 0 kernels, got %v", result)
	}
}

func TestExtractKernelsNilFields(t *testing.T) {
	releases := []*github.RepositoryRelease{
		{TagName: nil, Body: nil},
	}
	// GetTagName/GetBody return "" for nil fields, so this should not panic
	result := extractKernels(releases, "ubuntu22")
	if len(result) != 0 {
		t.Errorf("expected 0 kernels, got %v", result)
	}
}

func TestExtractKernelsMultipleReleasesMatchSameVariant(t *testing.T) {
	releases := []*github.RepositoryRelease{
		{TagName: strPtr("ubuntu22/20240101"), Body: strPtr("- Kernel Version: 6.5.0-44-generic\n")},
		{TagName: strPtr("ubuntu22/20240201"), Body: strPtr("- Kernel Version: 6.5.0-45-generic\n")},
	}

	result := extractKernels(releases, "ubuntu22")
	if len(result) != 2 {
		t.Fatalf("expected 2 kernels, got %d: %v", len(result), result)
	}
	if result[0] != "6.5.0-44-generic" {
		t.Errorf("expected 6.5.0-44-generic, got %s", result[0])
	}
	if result[1] != "6.5.0-45-generic" {
		t.Errorf("expected 6.5.0-45-generic, got %s", result[1])
	}
}
