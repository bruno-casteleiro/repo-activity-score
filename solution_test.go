package main

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommit_LineChanges(t *testing.T) {
	tests := []struct {
		name   string
		commit Commit
		want   int
	}{
		{
			name:   "zero value",
			commit: Commit{},
			want:   0,
		},
		{
			name:   "real data",
			commit: Commit{additions: 50, removals: 33},
			want:   83,
		},
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
		{
			name: "zero value",
			repo: Repo{},
			want: 0,
		},
		{
			name: "real data",
			repo: Repo{
				commits: []*Commit{{}, {}, {}},
			},
			want: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.repo.TotalCommits()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_TotalLineChanges(t *testing.T) {
	tests := []struct {
		name string
		repo Repo
		want int
	}{
		{
			name: "zero value",
			repo: Repo{},
			want: 0,
		},
		{
			name: "real data",
			repo: Repo{
				commits: []*Commit{
					{additions: 10, removals: 20},
					{additions: 150, removals: 5},
				},
			},
			want: 185,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.repo.TotalLineChanges()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_Consistency(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day := func(n int) time.Time { return base.AddDate(0, 0, n) }

	tests := []struct {
		name       string
		repo       Repo
		start, end time.Time
		want       float64
	}{
		{
			name: "zero date range",
			repo: Repo{commits: commitsOn(base)},
			want: 0,
		},
		{
			name:  "no commits",
			repo:  Repo{},
			start: base,
			end:   day(2),
			want:  0,
		},
		{
			// One commit every day in the range: perfectly even -> CV 0
			name:  "perfectly consistent",
			repo:  Repo{commits: commitsOn(base, day(1), day(2))},
			start: base,
			end:   day(2),
			want:  0,
		},
		{
			// All commits on the first of a 3-day range: daily counts
			// [3,0,0], mean 1, std sqrt(2) -> CV sqrt(2)
			name:  "single spike",
			repo:  Repo{commits: commitsOn(base, base, base)},
			start: base,
			end:   day(2),
			want:  math.Sqrt2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.repo.Consistency(tc.start, tc.end)
			assert.InDelta(t, tc.want, got, 1e-9)
		})
	}
}

func TestStats_Normalize(t *testing.T) {
	t.Run("no stats", func(t *testing.T) {
		stats := Stats{}

		got := stats.Normalize()

		assert.Nil(t, got.commits)
		assert.Nil(t, got.lineChanges)
		assert.Nil(t, got.consistency)
	})

	t.Run("real data", func(t *testing.T) {
		stats := Stats{
			commits:     []float64{1, 2, 3},    // min-max -> 0,50,100
			lineChanges: []float64{10, 10, 10}, // all equal -> 50,50,50
			consistency: []float64{0, 1, 2},    // inverted -> 100,50,0
		}

		got := stats.Normalize()

		require.Len(t, got.commits, 3)
		require.Len(t, got.lineChanges, 3)
		require.Len(t, got.consistency, 3)

		assert.InDeltaSlice(t, []float64{0, 50, 100}, got.commits, 1e-9)
		assert.InDeltaSlice(t, []float64{50, 50, 50}, got.lineChanges, 1e-9)
		assert.InDeltaSlice(t, []float64{100, 50, 0}, got.consistency, 1e-9)
	})
}

// Builds one commit per given date. Only the dates matter
// to Consistency, so additions/removals are left zero.
func commitsOn(dates ...time.Time) []*Commit {
	commits := make([]*Commit, 0, len(dates))
	for _, d := range dates {
		commits = append(commits, &Commit{date: d})
	}
	return commits
}
