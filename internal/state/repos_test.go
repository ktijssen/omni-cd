package state_test

import (
	"errors"
	"testing"

	"omni-cd/internal/config"
	"omni-cd/internal/state"
)

func TestApplyConfigRepos_AddsAndMarksFromConfig(t *testing.T) {
	s := newState()
	s.ApplyConfigRepos([]config.RepoConfig{
		{Name: "prod", URL: "https://github.com/example/prod.git", Branch: "main"},
	})
	repos := s.GetRepoConfigs()
	if len(repos) != 1 {
		t.Fatalf("want 1 repo, got %d", len(repos))
	}
	if !repos[0].FromConfig {
		t.Error("repo from config should have FromConfig=true")
	}
}

func TestApplyConfigRepos_OverridesExistingUIRepoByName(t *testing.T) {
	s := newState()
	if err := s.AddRepoConfig(config.RepoConfig{
		Name: "shared", URL: "https://github.com/example/old.git", Branch: "main",
	}); err != nil {
		t.Fatal(err)
	}
	s.ApplyConfigRepos([]config.RepoConfig{
		{Name: "shared", URL: "https://github.com/example/new.git", Branch: "develop"},
	})
	repos := s.GetRepoConfigs()
	if len(repos) != 1 {
		t.Fatalf("want 1 repo, got %d", len(repos))
	}
	if repos[0].URL != "https://github.com/example/new.git" {
		t.Errorf("config should override UI repo URL, got %q", repos[0].URL)
	}
	if !repos[0].FromConfig {
		t.Error("overridden repo should be marked FromConfig=true")
	}
}

func TestApplyConfigRepos_PreservesUnrelatedUIRepos(t *testing.T) {
	s := newState()
	if err := s.AddRepoConfig(config.RepoConfig{
		Name: "ui-only", URL: "https://github.com/example/ui.git", Branch: "main",
	}); err != nil {
		t.Fatal(err)
	}
	s.ApplyConfigRepos([]config.RepoConfig{
		{Name: "from-cfg", URL: "https://github.com/example/cfg.git", Branch: "main"},
	})

	byName := map[string]config.RepoConfig{}
	for _, r := range s.GetRepoConfigs() {
		byName[r.Name] = r
	}
	if len(byName) != 2 {
		t.Fatalf("want 2 repos, got %d", len(byName))
	}
	if byName["ui-only"].FromConfig {
		t.Error("UI-only repo should keep FromConfig=false")
	}
	if !byName["from-cfg"].FromConfig {
		t.Error("config-supplied repo should have FromConfig=true")
	}
}

func TestApplyConfigRepos_DemotesWhenRepoRemovedFromConfig(t *testing.T) {
	s := newState()
	// First boot: config provides "prod".
	s.ApplyConfigRepos([]config.RepoConfig{
		{Name: "prod", URL: "https://github.com/example/prod.git", Branch: "main"},
	})
	// Second boot: config no longer provides "prod" (still in state from first
	// boot because we don't drop existing repos). It should demote to UI-managed.
	s.ApplyConfigRepos(nil)
	repos := s.GetRepoConfigs()
	if len(repos) != 1 || repos[0].FromConfig {
		t.Errorf("repo should demote to UI-managed after removal from config; got %+v", repos)
	}
}

func TestDeleteRepoConfig_LockedRepoReturnsErrRepoLocked(t *testing.T) {
	s := newState()
	s.ApplyConfigRepos([]config.RepoConfig{
		{Name: "prod", URL: "https://github.com/example/prod.git", Branch: "main"},
	})
	err := s.DeleteRepoConfig("prod")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, state.ErrRepoLocked) {
		t.Errorf("expected ErrRepoLocked, got %v", err)
	}
	if len(s.GetRepoConfigs()) != 1 {
		t.Error("repo should not have been deleted")
	}
}

func TestUpdateRepoConfig_LockedRepoReturnsErrRepoLocked(t *testing.T) {
	s := newState()
	s.ApplyConfigRepos([]config.RepoConfig{
		{Name: "prod", URL: "https://github.com/example/prod.git", Branch: "main"},
	})
	err := s.UpdateRepoConfig("prod", config.RepoConfig{
		Name: "prod", URL: "https://github.com/example/changed.git", Branch: "main",
	})
	if !errors.Is(err, state.ErrRepoLocked) {
		t.Errorf("expected ErrRepoLocked, got %v", err)
	}
	if s.GetRepoConfigs()[0].URL == "https://github.com/example/changed.git" {
		t.Error("repo URL should not have been mutated")
	}
}

func TestDeleteRepoConfig_UIManagedStillWorks(t *testing.T) {
	s := newState()
	if err := s.AddRepoConfig(config.RepoConfig{
		Name: "ui", URL: "https://github.com/example/ui.git", Branch: "main",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRepoConfig("ui"); err != nil {
		t.Errorf("UI-managed repo should be deletable, got %v", err)
	}
	if len(s.GetRepoConfigs()) != 0 {
		t.Error("repo should have been removed")
	}
}
