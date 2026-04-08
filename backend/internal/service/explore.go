package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

// TagReader provides read access to global tag data.
type TagReader interface {
	GetTopTags(ctx context.Context, limit int) ([]store.TagPrevalence, error)
	GetRelatedTags(ctx context.Context, tag string, limit int) ([]store.TagRelation, error)
}

// ExploreService provides tag exploration capabilities for the explore graph.
type ExploreService struct {
	tags TagReader
}

// NewExploreService creates an ExploreService.
func NewExploreService(tags TagReader) (*ExploreService, error) {
	if tags == nil {
		return nil, errors.New("explore service: nil tag reader")
	}
	return &ExploreService{tags: tags}, nil
}

// GetTopVibes returns the most prevalent vibes across all cached artist data.
func (s *ExploreService) GetTopVibes(ctx context.Context, limit int) ([]store.TagPrevalence, error) {
	tags, err := s.tags.GetTopTags(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("querying top tags: %w", err)
	}
	return tags, nil
}

// GetRelatedVibes returns vibes related to a given tag by artist co-occurrence.
func (s *ExploreService) GetRelatedVibes(ctx context.Context, tag string, limit int) ([]store.TagRelation, error) {
	related, err := s.tags.GetRelatedTags(ctx, tag, limit)
	if err != nil {
		return nil, fmt.Errorf("querying related tags: %w", err)
	}
	return related, nil
}
