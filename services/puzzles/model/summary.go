package model

import "time"

// Summary is how lists, the feed and the single-puzzle response show a puzzle. Owner is nil when the account is gone.
type Summary struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Author     string    `json:"author,omitempty"`
	Source     string    `json:"source,omitempty"`
	Difficulty string    `json:"difficulty,omitempty"`
	Tags       []string  `json:"tags"`
	Variants   []string  `json:"variants"`
	Owner      *string   `json:"owner"`
	CreatedAt  time.Time `json:"createdAt"`
	RatingSummary
}

// RatingSummary is the rating of a puzzle as the client sees it. 0 means "no votes".
type RatingSummary struct {
	RatingCount     int     `json:"ratingCount"`
	Rating          float64 `json:"rating"`
	DifficultyCount int     `json:"difficultyCount"`
	FeltDifficulty  float64 `json:"feltDifficulty"`
}

// NewRatingSummary rounds the means to one decimal.
func NewRatingSummary(r RatingStats) RatingSummary {
	return RatingSummary{r.StarsCount, average(r.StarsSum, r.StarsCount), r.DifficultyCount, average(r.DifficultySum, r.DifficultyCount)}
}

func average(sum, count int) float64 {
	if count == 0 {
		return 0
	}

	return float64(sum*10/count) / 10
}

// NewSummary shows p with the owner's name from names, which maps owner ids to usernames.
func NewSummary(p *Puzzle, names map[int64]string) Summary {
	s := Summary{ID: p.ID, Title: p.Title, Author: p.Author, Source: p.Source, Difficulty: p.Difficulty,
		Tags: p.Tags, Variants: p.Variants, CreatedAt: p.CreatedAt, RatingSummary: NewRatingSummary(p.Ratings)}
	if name, ok := names[p.OwnerID]; ok {
		s.Owner = &name
	}

	return s
}
