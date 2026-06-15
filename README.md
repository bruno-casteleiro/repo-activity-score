# repo-activity-score

The `solution.go` program calculates a repository activity score (0-100) based on a provided dataset of GitHub commit data. The score measures the overall level of development activity by combining three independent dimensions of work: frequency, magnitude, and consistency.

## Algorithm Design

The activity score is calculated as a weighted combination of three normalized metrics:

```
Score = (0.33 × Commits Score) + (0.33 × Code Changes Score) + (0.34 × Consistency Score)
```

Each metric is normalized to a 0-100 scale, making them comparable despite different units and scales.

## Metrics

### 1. Total Commits

Measures the frequency of work — how often developers push code.

**Calculation:**
```
Total Commits = count of all commits in the repository
```

**Interpretation:**
- More commits = higher frequency of contributions

**Example:**
- Repo A: 150 commits → normalized score depends on other repos
- Repo B: 50 commits → will score lower if Repo A is the maximum

### 2. Total Line Changes

Measures the magnitude of work — how much code is being changed.

**Calculation:**
```
Total Line Changes = sum of (additions + deletions) across all commits in the repository
```

**Interpretation:**
- Higher line changes = more extensive code modifications
- Includes both feature development and refactoring/cleanup work
- Captures the "volume" of development effort

**Example:**
- A 1000-line refactor = same weight as 1000 new feature lines
- Values both active development and maintenance work

### 3. Commit Consistency

Measures the steadiness of activity — how evenly commits are distributed over time.

**Calculation:**
```
1. Count commits per day for the analysis period
2. Calculate standard deviation of daily commit counts
3. Calculate mean = total commits / number of days
4. Consistency = standard deviation / mean  (Coefficient of Variation)
5. Normalize to 0-100 (inverted: lower variability = higher score)
```

**Interpretation:**
- Coefficient of Variation (CV) measures relative variability
  - CV = 0: perfectly consistent (same number of commits every day)
  - CV > 1: very sporadic (large swings in daily activity)
- Lower CV = healthier, more sustainable activity pattern
- Inverted during normalization so higher scores are better

**Example:**
- Repo A: 1 commit/day for 100 days → CV ≈ 0 → High consistency score
- Repo B: 100 commits on day 50, 0 elsewhere → CV ≈ 4.5 → Low consistency score
- Repo C: 2 commits/day consistently → CV ≈ 0 → High consistency score (same as A)

## Weight Distribution

The weights are equally distributed across the three dimensions:

| Metric | Weight | Rationale |
|--------|--------|-----------|
| Total Commits | 0.33 | Frequency of work matters equally |
| Total Line Changes | 0.33 | Magnitude of work matters equally |
| Commit Consistency | 0.34 | Steadiness as slight tiebreaker |

**Interpretation:**
All three dimensions are equally important for measuring repository activity:
- A repository with many small commits is as active as one with few large commits
- Steady, sustained work is slightly preferred over sporadic bursts
- The 0.34 on consistency acts as a gentle tiebreaker between repos with similar commit frequency and volume

## Running the Algorithm

### Prerequisites
- Go 1.22 or later
- CSV file with commit data in the format specified in the task instructions

### Basic Usage

```bash
go run solution.go -f commits.csv
```

This will:
- Load `commits.csv`
- Use default weights (0.33, 0.33, 0.34)
- Display top 10 repositories

### Advanced Usage

#### Custom weights:
```bash
go run solution.go -f commits.csv \
  -w-commits 0.4 \
  -w-changes 0.5 \
  -w-consistency 0.1
```

#### Custom top-X results:
```bash
go run solution.go -f commits.csv -t 20
```

Shows top 20 repositories instead of 10.

### Available Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-f` | (required) | Path to CSV file |
| `-w-commits` | 0.4 | Weight for commit frequency (0-1) |
| `-w-changes` | 0.5 | Weight for line changes (0-1) |
| `-w-consistency` | 0.1 | Weight for consistency (0-1) |
| `-t` | 10 | Number of top repos to display |

**Note:** If weights don't sum to 1.0, they are automatically normalized.


## Top 10 Most Active Repositories

Below follows the top-10 results for the `commits.csv` file provided, using the algorithm's default weights:

| Rank | Repository | Activity Score |
|------|------------|-----------------|
| 1 | repo476 | 77 |
| 2 | repo250 | 69 |
| 3 | repo518 | 55 |
| 4 | repo740 | 52 |
| 5 | repo126 | 51 |
| 6 | repo795 | 51 |
| 7 | repo127 | 47 |
| 8 | repo982 | 47 |
| 9 | repo703 | 45 |
| 10 | repo117 | 42 |
