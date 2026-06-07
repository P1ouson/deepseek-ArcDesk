package main

import (
	"net/url"
	"testing"
)

func TestValidatePreviewURL_NonStrictLocalhost(t *testing.T) {
	profile := defaultProjectSandboxProfile("/tmp/proj")
	got := validatePreviewURL("http://localhost:5173", profile, false)
	if got.Decision != "allow" {
		t.Fatalf("decision = %q, want allow", got.Decision)
	}
}

func TestValidatePreviewURL_NonStrictExternalConfirm(t *testing.T) {
	profile := defaultProjectSandboxProfile("/tmp/proj")
	got := validatePreviewURL("https://example.com", profile, false)
	if got.Decision != "confirm" {
		t.Fatalf("decision = %q, want confirm", got.Decision)
	}
}

func TestValidatePreviewURL_StrictWhitelist(t *testing.T) {
	profile := ProjectSandboxProfile{
		Configured:   true,
		PreviewHosts: []string{"localhost", "127.0.0.1"},
		PreviewPorts: []int{5173},
	}
	allow := validatePreviewURL("http://localhost:5173", profile, true)
	if allow.Decision != "allow" {
		t.Fatalf("localhost decision = %q, want allow", allow.Decision)
	}
	block := validatePreviewURL("https://example.com", profile, true)
	if block.Decision != "blocked" || block.Reason != "not-in-profile" {
		t.Fatalf("external decision = %+v, want blocked/not-in-profile", block)
	}
}

func TestValidatePreviewURL_BlocksUnsafeScheme(t *testing.T) {
	profile := defaultProjectSandboxProfile("/tmp/proj")
	got := validatePreviewURL("javascript:alert(1)", profile, false)
	if got.Decision != "blocked" || got.Reason != "unsafe-scheme" {
		t.Fatalf("got %+v, want blocked unsafe-scheme", got)
	}
}

func TestNormalizePreviewHosts_RejectsWildcards(t *testing.T) {
	got := normalizePreviewHosts([]string{"localhost", "*.local", "127.0.0.1", ".example.com"})
	if len(got) != 2 {
		t.Fatalf("normalizePreviewHosts = %v, want [localhost 127.0.0.1]", got)
	}
}

func TestPreviewAllowedByProfile_ExactHostOnly(t *testing.T) {
	profile := ProjectSandboxProfile{
		Configured:   true,
		PreviewHosts: []string{"localhost"},
		PreviewPorts: []int{5173},
	}
	u, _ := url.Parse("http://app.localhost:5173")
	if previewAllowedByProfile(u, profile) {
		t.Fatal("subdomain should not match exact localhost host")
	}
}
