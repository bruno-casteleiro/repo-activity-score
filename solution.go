package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"time"
)

type Commit struct {
	date      time.Time
	additions int
	removals  int
}

type Repo struct {
	name    string
	commits []*Commit
}

type Stats struct {
	commits     []float64 // Total commits per repo
	lineChanges []float64 // Total lines changed per repo
	consistency []float64 // Commit distribution over time per repo
}

type RepoScore struct {
	name  string
	score int
}

// Returns the total number of lines changed in a commit.
func (c Commit) LineChanges() int {
	return c.additions + c.removals
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

	dailyCommitsCount := []int{}
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

// Applies min-max normalization to scale values to 0-100.
// Formula: ((value - min) / (max - min)) * 100
func normalizeScore(values []float64) []float64 {
	if len(values) == 0 {
		return []float64{}
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

// Creates a new commit using string values from a CSV row.
func makeCommit(timestamp, additions, removals string) *Commit {
	intTimestamp, err1 := strconv.ParseInt(timestamp, 10, 64)
	intAdditions, err2 := strconv.Atoi(additions)
	intRemovals, err3 := strconv.Atoi(removals)

	// skip bad rows
	if err1 != nil || err2 != nil || err3 != nil {
		return nil
	}

	return &Commit{time.Unix(intTimestamp, 0), intAdditions, intRemovals}
}

// Creates a new repo with an initial commit.
func makeRepo(name string, commit *Commit) *Repo {
	return &Repo{name, []*Commit{commit}}
}

func main() {
	// Define CLI flags for parameterization
	filename := flag.String("f", "", "Path to CSV file (required)")
	wCommits := flag.Float64("w-commits", 0.33, "Weight for 'total commits' metric (0-1)")
	wChanges := flag.Float64("w-changes", 0.33, "Weight for 'total line changes' metric (0-1)")
	wConsistency := flag.Float64("w-consistency", 0.34, "Weight for 'commit consistency' metric (0-1)")
	top := flag.Int("t", 10, "Show rank for top-x repos")

	flag.Parse()

	// Validate filename
	if *filename == "" {
		fmt.Println("Error: filename must be provided")
		os.Exit(1)
	}

	// Validate weights
	if *wCommits < 0 || *wChanges < 0 || *wConsistency < 0 {
		fmt.Println("Error: weights must be non-negative")
		os.Exit(1)
	}

	// Normalize weights to sum to 1.0
	totalWeight := *wCommits + *wChanges + *wConsistency
	if totalWeight == 0 {
		fmt.Println("Error: total weight must be greater than 0")
		os.Exit(1)
	}

	w1 := *wCommits / totalWeight
	w2 := *wChanges / totalWeight
	w3 := *wConsistency / totalWeight

	// Validate top-x
	if *top <= 0 {
		fmt.Println("Error: top must be positive")
		os.Exit(1)
	}

	// Open and read CSV file
	file, err := os.Open(*filename)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Parse CSV and group commits by repo
	// Expected CSV format:
	// 		[0]=timestamp, [1]=user, [2]=repository, [3]=files_changed, [4]=additions, [5]=removals
	repos := make(map[string]*Repo)
	repoNames := []string{}
	oldestCommitDate := time.Now()
	latestCommitDate := time.Unix(0, 0)

	for _, record := range records[1:] {
		commit := makeCommit(record[0], record[4], record[5])
		if commit == nil {
			continue
		}

		r, ok := repos[record[2]]
		if ok {
			r.commits = append(r.commits, commit)
		} else {
			repos[record[2]] = makeRepo(record[2], commit)
			repoNames = append(repoNames, record[2])
		}

		// Track oldest/latest commit date for later use
		if commit.date.Before(oldestCommitDate) {
			oldestCommitDate = commit.date
		}
		if commit.date.After(latestCommitDate) {
			latestCommitDate = commit.date
		}
	}

	// Calculate raw metrics for each repo
	stats := Stats{}
	for _, repo := range repoNames {
		repo := repos[repo]
		stats.commits = append(stats.commits, float64(repo.TotalCommits()))
		stats.lineChanges = append(stats.lineChanges, float64(repo.TotalLineChanges()))
		stats.consistency = append(stats.consistency, repo.Consistency(oldestCommitDate, latestCommitDate))
	}

	// Normalize metrics to 0-100 scale
	normalizedStats := stats.Normalize()

	// Calculate weighted activity score for each repo
	repoScores := []RepoScore{}
	for idx, repo := range repoNames {
		score := (normalizedStats.commits[idx] * w1) + (normalizedStats.lineChanges[idx] * w2) + (normalizedStats.consistency[idx] * w3)
		repoScores = append(repoScores, RepoScore{repo, int(math.Round(score))})
	}

	// Sort repos by score in descending order
	sort.Slice(repoScores, func(i, j int) bool {
		return repoScores[i].score > repoScores[j].score
	})

	// Display top-x rank
	topx := min(*top, len(repoScores))
	fmt.Printf("Top-%d most active repos:\n", topx)
	for pos := range topx {
		repoScore := repoScores[pos]
		fmt.Printf("%d: %s (%d)\n", pos+1, repoScore.name, repoScore.score)
	}
}
