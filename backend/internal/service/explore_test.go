package service

import (
	"context"
	"errors"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

type mockTagReader struct {
	topTags     []store.TagPrevalence
	relatedTags []store.TagRelation
	topErr      error
	relatedErr  error
}

func (m *mockTagReader) GetTopTags(_ context.Context, _ int) ([]store.TagPrevalence, error) {
	return m.topTags, m.topErr
}

func (m *mockTagReader) GetRelatedTags(_ context.Context, _ string, _ int) ([]store.TagRelation, error) {
	return m.relatedTags, m.relatedErr
}

func TestExploreService_GetTopVibes(t *testing.T) {
	svc, _ := NewExploreService(&mockTagReader{
		topTags: []store.TagPrevalence{
			{Tag: "rock", Prevalence: 1.0},
			{Tag: "indie", Prevalence: 0.85},
		},
	})

	tags, err := svc.GetTopVibes(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetTopVibes: %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestExploreService_GetTopVibes_Error(t *testing.T) {
	svc, _ := NewExploreService(&mockTagReader{topErr: errors.New("db error")})

	_, err := svc.GetTopVibes(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExploreService_GetRelatedVibes(t *testing.T) {
	svc, _ := NewExploreService(&mockTagReader{
		relatedTags: []store.TagRelation{
			{Tag: "indie", Strength: 1.0},
		},
	})

	related, err := svc.GetRelatedVibes(context.Background(), "rock", 8)
	if err != nil {
		t.Fatalf("GetRelatedVibes: %v", err)
	}
	if len(related) != 1 {
		t.Errorf("expected 1 related tag, got %d", len(related))
	}
}

func TestExploreService_GetRelatedVibes_Error(t *testing.T) {
	svc, _ := NewExploreService(&mockTagReader{relatedErr: errors.New("db error")})

	_, err := svc.GetRelatedVibes(context.Background(), "rock", 8)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewExploreService_NilTagReader(t *testing.T) {
	_, err := NewExploreService(nil)
	if err == nil {
		t.Error("expected error for nil tag reader")
	}
}
