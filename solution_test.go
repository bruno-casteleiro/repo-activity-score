package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommit_lineChanges(t *testing.T) {
	tests := []struct {
		name   string
		commit commit
		want   int
	}{
		{
			name:   "zero value",
			commit: commit{},
			want:   0,
		},
		{
			name:   "real data",
			commit: commit{additions: 50, removals: 33},
			want:   83,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.commit.lineChanges()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_totalCommits(t *testing.T) {
	tests := []struct {
		name string
		repo repo
		want int
	}{
		{
			name: "zero value",
			repo: repo{},
			want: 0,
		},
		{
			name: "real data",
			repo: repo{
				commits: []*commit{{}, {}, {}},
			},
			want: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.repo.totalCommits()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_totalLineChanges(t *testing.T) {
	tests := []struct {
		name string
		repo repo
		want int
	}{
		{
			name: "zero value",
			repo: repo{},
			want: 0,
		},
		{
			name: "real data",
			repo: repo{
				commits: []*commit{
					{additions: 10, removals: 20},
					{additions: 150, removals: 5},
				},
			},
			want: 185,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.repo.totalLineChanges()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_consistency(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day := func(n int) time.Time { return base.AddDate(0, 0, n) }

	tests := []struct {
		name       string
		repo       repo
		start, end time.Time
		want       float64
	}{
		{
			name: "zero date range",
			repo: repo{commits: commitsOn(base)},
			want: 0,
		},
		{
			name:  "no commits",
			repo:  repo{},
			start: base,
			end:   day(2),
			want:  0,
		},
		{
			// One commit every day in the range: perfectly even -> CV 0
			name:  "perfectly consistent",
			repo:  repo{commits: commitsOn(base, day(1), day(2))},
			start: base,
			end:   day(2),
			want:  0,
		},
		{
			// All commits on the first of a 3-day range: daily counts
			// [3,0,0], mean 1, std sqrt(2) -> CV sqrt(2)
			name:  "single spike",
			repo:  repo{commits: commitsOn(base, base, base)},
			start: base,
			end:   day(2),
			want:  math.Sqrt2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.repo.consistency(tc.start, tc.end)
			assert.InDelta(t, tc.want, got, 1e-9)
		})
	}
}

func TestStats_normalize(t *testing.T) {
	t.Run("no stats", func(t *testing.T) {
		stats := stats{}

		got := stats.normalize()

		assert.Nil(t, got.commits)
		assert.Nil(t, got.lineChanges)
		assert.Nil(t, got.consistency)
	})

	t.Run("real data", func(t *testing.T) {
		stats := stats{
			commits:     []float64{1, 2, 3},    // min-max -> 0,50,100
			lineChanges: []float64{10, 10, 10}, // all equal -> 50,50,50
			consistency: []float64{0, 1, 2},    // inverted -> 100,50,0
		}

		got := stats.normalize()

		require.Len(t, got.commits, 3)
		require.Len(t, got.lineChanges, 3)
		require.Len(t, got.consistency, 3)

		assert.InDeltaSlice(t, []float64{0, 50, 100}, got.commits, 1e-9)
		assert.InDeltaSlice(t, []float64{50, 50, 50}, got.lineChanges, 1e-9)
		assert.InDeltaSlice(t, []float64{100, 50, 0}, got.consistency, 1e-9)
	})
}

func Test_run(t *testing.T) {
	const csv = `timestamp,user,repository,files_changed,additions,removals
1700000000,alice,repo-a,1,100,0
1700086400,alice,repo-a,1,100,0
1700172800,alice,repo-a,1,100,0
1700000000,bob,repo-b,1,1,0
`

	t.Run("happy path ranks repos by score", func(t *testing.T) {
		path := writeCSV(t, csv)
		var out bytes.Buffer

		err := run([]string{"-f", path}, &out)

		require.NoError(t, err)
		assert.Equal(t,
			"Top-2 most active repos:\n1: repo-a (100)\n2: repo-b (0)\n",
			out.String())
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		var out bytes.Buffer

		err := run([]string{}, &out) // missing required -f

		require.Error(t, err)
		assert.Empty(t, out.String())
	})

	t.Run("missing file returns error", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does-not-exist.csv")
		var out bytes.Buffer

		err := run([]string{"-f", missing}, &out)

		require.Error(t, err)
		assert.Empty(t, out.String())
	})

	t.Run("empty csv returns error", func(t *testing.T) {
		path := writeCSV(t, "")
		var out bytes.Buffer

		err := run([]string{"-f", path}, &out)

		require.Error(t, err)
	})

	t.Run("top larger than repo count is clamped", func(t *testing.T) {
		path := writeCSV(t, csv)
		var out bytes.Buffer

		err := run([]string{"-f", path, "-t", "100"}, &out)

		require.NoError(t, err)
		assert.Contains(t, out.String(), "Top-2 most active repos:")
	})
}

func Test_parseConfig(t *testing.T) {
	t.Run("parse error returns error", func(t *testing.T) {
		_, err := parseConfig([]string{"-bogus"})
		require.Error(t, err)
	})

	t.Run("no filename returns error", func(t *testing.T) {
		_, err := parseConfig([]string{"-f"})
		require.Error(t, err)
	})

	t.Run("negative weights returns error", func(t *testing.T) {
		t.Run("w-commits", func(t *testing.T) {
			_, err := parseConfig([]string{"-f", "filename", "-w-commits", "-0.5"})
			require.Error(t, err)
		})

		t.Run("w-changes", func(t *testing.T) {
			_, err := parseConfig([]string{"-f", "filename", "-w-changes", "-0.5"})
			require.Error(t, err)
		})

		t.Run("w-consistency", func(t *testing.T) {
			_, err := parseConfig([]string{"-f", "filename", "-w-consistency", "-0.5"})
			require.Error(t, err)
		})
	})

	t.Run("zero total weight returns error", func(t *testing.T) {
		_, err := parseConfig([]string{"-f", "filename", "-w-commits", "0", "-w-changes", "0", "-w-consistency", "0"})
		require.Error(t, err)
	})

	t.Run("invalid top returns returns error", func(t *testing.T) {
		t.Run("zero", func(t *testing.T) {
			_, err := parseConfig([]string{"-f", "filename", "-t", "0"})
			require.Error(t, err)
		})

		t.Run("negative", func(t *testing.T) {
			_, err := parseConfig([]string{"-f", "filename", "-t", "-10"})
			require.Error(t, err)
		})
	})

	t.Run("valid args", func(t *testing.T) {
		config, err := parseConfig([]string{"-f", "filename", "-w-commits", "1", "-w-changes", "2", "-w-consistency", "3", "-t", "20"})

		require.NoError(t, err)

		assert.Equal(t, "filename", config.filename)
		assert.InDelta(t, 1/6.0, config.weights.commits, 1e-9)
		assert.InDelta(t, 2/6.0, config.weights.changes, 1e-9)
		assert.InDelta(t, 3/6.0, config.weights.consistency, 1e-9)
		assert.Equal(t, 20, config.top)
	})
}

// Builds one commit per given date. Only the dates matter
// to Consistency, so additions/removals are left zero.
func commitsOn(dates ...time.Time) []*commit {
	commits := make([]*commit, 0, len(dates))
	for _, d := range dates {
		commits = append(commits, &commit{date: d})
	}
	return commits
}

func writeCSV(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "in.csv")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}
