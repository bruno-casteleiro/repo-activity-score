package main

import (
	"cmp"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"time"
)

type Config struct {
	filename string
	weights  Weights
	top      int
}

type Weights struct {
	commits, changes, consistency float64
}

type Commit struct {
	date      time.Time
	additions int
	removals  int
}

// Returns the total number of lines changed in a commit.
func (c Commit) LineChanges() int {
	return c.additions + c.removals
}

type Repo struct {
	name    string
	commits []*Commit
}

// Returns the number of commits in a repo.
func (r Repo) TotalCommits() int {
	return len(r.commits)
}

// Returns the sum of all line changes across all commits in a repo.
func (r Repo) TotalLineChanges() int {
	result := 0
	for _, commit := range r.commits {
		result += commit.LineChanges()
	}
	return result
}

// Returns a measure of how evenly commits are distributed over time.
// Uses Coefficient of Variation to measure relative variability.
// Lower CV = more consistent activity, Higher CV = more sporadic activity.
func (r Repo) Consistency(startDate time.Time, endDate time.Time) float64 {
	commitsPerDay := make(map[string]int)
	for _, commit := range r.commits {
		day := commit.date.Format("02-01-2006")
		commitsPerDay[day]++
	}

	var dailyCommitsCount []int
	cur := startDate
	end := endDate.AddDate(0, 0, 1)

	for cur.Before(end) {
		day := cur.Format("02-01-2006")
		dailyCommitsCount = append(dailyCommitsCount, commitsPerDay[day])
		cur = cur.AddDate(0, 0, 1)
	}

	// Calculate coefficient of variation: std dev / mean
	mean := float64(r.TotalCommits()) / float64(len(dailyCommitsCount))
	if mean == 0 {
		return 0
	}

	return std(dailyCommitsCount) / mean
}

type Stats struct {
	commits     []float64 // Total commits per repo
	lineChanges []float64 // Total lines changed per repo
	consistency []float64 // Commit distribution over time per repo
}

// Transforms raw metrics into normalized scores (0-100).
func (s Stats) Normalize() Stats {
	// Find max consistency value for inversion
	maxConsistency := 0.0
	for _, v := range s.consistency {
		if v > maxConsistency {
			maxConsistency = v
		}
	}

	// Invert consistency: higher CV (sporadic activity) becomes lower score
	invertedStdDailyCommits := make([]float64, len(s.consistency))
	for idx, v := range s.consistency {
		invertedStdDailyCommits[idx] = maxConsistency - v
	}

	return Stats{
		commits:     normalizeScore(s.commits),
		lineChanges: normalizeScore(s.lineChanges),
		consistency: normalizeScore(invertedStdDailyCommits),
	}
}

type RepoScore struct {
	name  string
	score int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {

	cfg, err := parseConfig()
	if err != nil {
		return err
	}

	file, err := os.Open(cfg.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	repos, oldestCommit, latestCommit, err := parseRepos(file)
	if err != nil {
		return err
	}

	scores := scoreRepos(repos, oldestCommit, latestCommit, cfg.weights)

	// Display top-x rank
	topx := min(cfg.top, len(scores))
	fmt.Printf("Top-%d most active repos:\n", topx)
	for pos := range topx {
		repoScore := scores[pos]
		fmt.Printf("%d: %s (%d)\n", pos+1, repoScore.name, repoScore.score)
	}

	return nil
}

func parseConfig() (Config, error) {
	// Define CLI flags for parameterization
	filename := flag.String("f", "", "Path to CSV file (required)")
	wCommits := flag.Float64("w-commits", 0.33, "Weight for 'total commits' metric (0-1)")
	wChanges := flag.Float64("w-changes", 0.33, "Weight for 'total line changes' metric (0-1)")
	wConsistency := flag.Float64("w-consistency", 0.34, "Weight for 'commit consistency' metric (0-1)")
	top := flag.Int("t", 10, "Show rank for top-x repos")

	flag.Parse()

	// Validate filename
	if *filename == "" {
		return Config{}, errors.New("filename must be provided")
	}

	// Validate weights
	if *wCommits < 0 || *wChanges < 0 || *wConsistency < 0 {
		return Config{}, errors.New("weights must be non-negative")
	}

	totalWeight := *wCommits + *wChanges + *wConsistency
	if totalWeight == 0 {
		return Config{}, errors.New("sum of weights must be greater than 0")
	}

	// Validate top-x
	if *top <= 0 {
		return Config{}, errors.New("top must be positive")
	}

	return Config{
		*filename,
		Weights{
			// weights are normalized
			*wCommits / totalWeight,
			*wChanges / totalWeight,
			*wConsistency / totalWeight,
		},
		*top,
	}, nil
}

func parseRepos(r io.Reader) (repos []*Repo, oldestCommit, latestCommit time.Time, err error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return
	}

	if len(records) == 0 {
		err = errors.New("CSV file provided is empty")
		return
	}

	// Parse CSV and group commits by repo
	// Expected CSV format:
	// 		[0]=timestamp, [1]=user, [2]=repository, [3]=files_changed, [4]=additions, [5]=removals
	seen := make(map[string]*Repo)
	first := true
	for _, record := range records[1:] {
		if len(record) < 6 {
			continue
		}

		commit, err := newCommit(record[0], record[4], record[5])
		if err != nil {
			continue
		}

		name := record[2]

		if repo, ok := seen[name]; ok {
			repo.commits = append(repo.commits, commit)
		} else {
			repo = newRepo(name, commit)
			seen[name] = repo
			repos = append(repos, repo)
		}

		// Seed the date range on the first valid commit, then widen it.
		switch {
		case first:
			oldestCommit, latestCommit = commit.date, commit.date
			first = false
		case commit.date.Before(oldestCommit):
			oldestCommit = commit.date
		case commit.date.After(latestCommit):
			latestCommit = commit.date
		}
	}

	return
}

// Creates a new commit using string values from a CSV row.
func newCommit(timestamp, additions, removals string) (*Commit, error) {
	intTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp %q: %w", timestamp, err)
	}

	intAdditions, err := strconv.Atoi(additions)
	if err != nil {
		return nil, fmt.Errorf("parse additions %q: %w", additions, err)
	}

	intRemovals, err := strconv.Atoi(removals)
	if err != nil {
		return nil, fmt.Errorf("parse removals %q: %w", removals, err)
	}

	return &Commit{time.Unix(intTimestamp, 0), intAdditions, intRemovals}, nil
}

// Creates a new repo with an initial commit.
func newRepo(name string, commit *Commit) *Repo {
	return &Repo{name, []*Commit{commit}}
}

func scoreRepos(repos []*Repo, oldestCommit, latestCommit time.Time, weights Weights) []RepoScore {
	// Calculate raw metrics for each repo
	stats := Stats{}
	for _, repo := range repos {
		stats.commits = append(stats.commits, float64(repo.TotalCommits()))
		stats.lineChanges = append(stats.lineChanges, float64(repo.TotalLineChanges()))
		stats.consistency = append(stats.consistency, repo.Consistency(oldestCommit, latestCommit))
	}

	// Normalize metrics to 0-100 scale
	normalizedStats := stats.Normalize()

	// Calculate weighted activity score for each repo
	var repoScores []RepoScore
	for idx, repo := range repos {
		score := (normalizedStats.commits[idx] * weights.commits) + (normalizedStats.lineChanges[idx] * weights.changes) + (normalizedStats.consistency[idx] * weights.consistency)
		repoScores = append(repoScores, RepoScore{repo.name, int(math.Round(score))})
	}

	// Sort repos by score in descending order
	slices.SortFunc(repoScores, func(a, b RepoScore) int {
		scoreCmp := cmp.Compare(a.score, b.score)
		if scoreCmp == 0 {
			return cmp.Compare(a.name, b.name)
		}

		return -scoreCmp
	})

	return repoScores
}

// Calculates the standard deviation of a set of integers.
func std(values []int) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	sum := 0
	for _, v := range values {
		sum += v
	}
	mean := float64(sum) / float64(len(values))

	// Calculate variance
	variance := 0.0
	for _, v := range values {
		diff := float64(v) - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return math.Sqrt(variance)
}

// Applies min-max normalization to scale values to 0-100.
// Formula: ((value - min) / (max - min)) * 100
func normalizeScore(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}

	minVal := values[0]
	maxVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	result := make([]float64, len(values))

	// Retrieve middle score if all values are equal
	if minVal == maxVal {
		for i := range result {
			result[i] = 50.0
		}
		return result
	}

	// Normalize to 0-100 range
	for i, v := range values {
		result[i] = ((v - minVal) / (maxVal - minVal)) * 100
	}

	return result
}
