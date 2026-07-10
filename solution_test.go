package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommit_LineChanges(t *testing.T) {
	tests := []struct {
		name   string
		commit Commit
		want   int
	}{
		{"zero value", Commit{}, 0},
		{"real data", Commit{additions: 50, removals: 33}, 83},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.commit.LineChanges()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_TotalCommits(t *testing.T) {
	tests := []struct {
		name string
		repo Repo
		want int
	}{
		{"zero value", Repo{}, 0},
		{"real data", Repo{commits: []*Commit{{}, {}, {}}}, 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.repo.TotalCommits()
			assert.Equal(t, tc.want, got)
		})
	}
}
