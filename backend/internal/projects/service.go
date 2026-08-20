// Package projects implements the project domain: projects, integration keys.
package projects

import (
	"time"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/audit"
	"cashx/internal/platform"
	"cashx/internal/repository"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,38}[a-z0-9])?$`)

// Service implements project operations.
type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.Recorder
}

func (s *Service) q(ctx context.Context) *repository.Queries { return repository.New(s.Pool) }

// Card is the API shape of a project.
type Card struct {
	ID             string  `json:"id"`
	Slug           string  `json:"slug"`
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	DestinationURL string  `json:"destination_url"`
	IsActive       bool    `json:"is_active"`
	CreatedAt      string  `json:"created_at"`
}

func rowCard(id, slug, name string, description *string, destinationURL string, isActive bool, createdAt time.Time) Card {
	return Card{
		ID:             id,
		Slug:           slug,
		Name:           name,
		Description:    description,
		DestinationURL: destinationURL,
		IsActive:       isActive,
		CreatedAt:      createdAt.UTC().Format(time.RFC3339),
	}
}

// Create validates and inserts a project.
func (s *Service) Create(ctx context.Context, actorID *string, slug, name, description, destinationURL string, isActive bool) (Card, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	name = strings.TrimSpace(name)
	destinationURL = strings.TrimSpace(destinationURL)
	if !slugRe.MatchString(slug) {
		return Card{}, fmt.Errorf("%w: invalid_slug", platform.ErrValidation)
	}
	if name == "" || destinationURL == "" {
		return Card{}, fmt.Errorf("%w: invalid_project", platform.ErrValidation)
	}
	var card Card
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		p, err := tq.CreateProject(ctx, repository.CreateProjectParams{
			Slug:           slug,
			Name:           name,
			Description:    repository.TextPtr(nilIfEmpty(description)),
			DestinationUrl: destinationURL,
			IsActive:       isActive,
		})
		if err != nil {
			return err
		}
		if err := tq.UpsertProjectSettings(ctx, p.ID); err != nil {
			return err
		}
		card = rowCard(p.ID, p.Slug, p.Name, repository.TextToPtr(p.Description), p.DestinationUrl, p.IsActive, p.CreatedAt.Time)
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Card{}, fmt.Errorf("%w: slug_taken", platform.ErrConflict)
		}
		return Card{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "project.created", "project", card.ID, map[string]any{"slug": slug}, nil)
	}
	return card, nil
}

// Update patches project fields.
func (s *Service) Update(ctx context.Context, actorID *string, id string, name, description, destinationURL *string, isActive *bool) (Card, error) {
	var card Card
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		p, err := tq.UpdateProject(ctx, repository.UpdateProjectParams{
			ID:             id,
			Name:           repository.TextPtr(name),
			Description:    repository.TextPtr(description),
			DestinationUrl: repository.TextPtr(destinationURL),
			IsActive:       repository.BoolPtr(isActive),
		})
		if err != nil {
			return err
		}
		card = rowCard(p.ID, p.Slug, p.Name, repository.TextToPtr(p.Description), p.DestinationUrl, p.IsActive, p.CreatedAt.Time)
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, fmt.Errorf("%w: project_not_found", platform.ErrNotFound)
		}
		return Card{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "project.updated", "project", card.ID, map[string]any{"name": name, "is_active": isActive}, nil)
	}
	return card, nil
}

// List returns projects with pagination.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Card, int64, error) {
	rows, err := s.q(ctx).ListProjects(ctx, repository.ListProjectsParams{Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q(ctx).CountProjects(ctx)
	if err != nil {
		return nil, 0, err
	}
	cards := make([]Card, 0, len(rows))
	for _, r := range rows {
		cards = append(cards, rowCard(r.ID, r.Slug, r.Name, repository.TextToPtr(r.Description), r.DestinationUrl, r.IsActive, r.CreatedAt.Time))
	}
	return cards, total, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
