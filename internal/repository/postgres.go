package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested record does not exist
var ErrNotFound = errors.New("record not found")

// Repository handles database operations for images, jobs, and variants
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new database repository instance
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

// CreateImage inserts a new image record and populates its generated ID and CreatedAt
func (r *Repository) CreateImage(ctx context.Context, img *domain.Image) error {
	query := `
		INSERT INTO images (original_path, original_format, width, height, file_size_bytes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at;
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		img.OriginalPath,
		img.OriginalFormat,
		img.Width,
		img.Height,
		img.FileSizeBytes,
	).Scan(&img.ID, &img.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert image: %w", err)
	}

	return nil
}

// CreateJob inserts a new processing job and populates its generated ID, CreatedAt, and UpdatedAt
func (r *Repository) CreateJob(ctx context.Context, job *domain.Job) error {
	query := `
		INSERT INTO jobs (image_id, status, attempts, max_attempts)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at;
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		job.ImageID,
		job.Status,
		job.Attempts,
		job.MaxAttempts,
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}

	return nil
}

// GetJobByID retrieves a job by its unique identifier
func (r *Repository) GetJobByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	query := `
		SELECT id, image_id, status, attempts, max_attempts, error_message, created_at, updated_at
		FROM jobs
		WHERE id = $1;
	`

	var job domain.Job
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.ImageID,
		&job.Status,
		&job.Attempts,
		&job.MaxAttempts,
		&job.ErrorMessage,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get job by id: %w", err)
	}

	return &job, nil
}

// GetImageByID retrieves image metadata by its unique identifier
func (r *Repository) GetImageByID(ctx context.Context, id uuid.UUID) (*domain.Image, error) {
	query := `
		SELECT id, original_path, original_format, width, height, file_size_bytes, created_at
		FROM images
		WHERE id = $1;
	`

	var img domain.Image
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&img.ID,
		&img.OriginalPath,
		&img.OriginalFormat,
		&img.Width,
		&img.Height,
		&img.FileSizeBytes,
		&img.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get image by id: %w", err)
	}

	return &img, nil
}

// UpdateJob updates job status, attempts count, error message and sets updated_at to now
func (r *Repository) UpdateJob(ctx context.Context, id uuid.UUID, status string, attempts int, errorMessage *string) error {
	query := `
		UPDATE jobs
		SET status = $2,
		    attempts = $3,
		    error_message = $4,
		    updated_at = NOW()
		WHERE id = $1;
	`

	cmdTag, err := r.pool.Exec(ctx, query, id, status, attempts, errorMessage)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// CreateVariants inserts generated image variants within a single transaction
// Uses ON CONFLICT to remain idempotent if a worker retries the task
func (r *Repository) CreateVariants(ctx context.Context, variants []domain.ImageVariant) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO image_variants (image_id, size_type, format, path, width, height, file_size_bytes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (image_id, size_type, format) DO UPDATE SET
			path = EXCLUDED.path,
			width = EXCLUDED.width,
			height = EXCLUDED.height,
			file_size_bytes = EXCLUDED.file_size_bytes
		RETURNING id, created_at;
	`

	for i := range variants {
		err := tx.QueryRow(
			ctx,
			query,
			variants[i].ImageID,
			variants[i].SizeType,
			variants[i].Format,
			variants[i].Path,
			variants[i].Width,
			variants[i].Height,
			variants[i].FileSizeBytes,
		).Scan(&variants[i].ID, &variants[i].CreatedAt)

		if err != nil {
			return fmt.Errorf("failed to insert variant %s/%s: %w", variants[i].SizeType, variants[i].Format, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit variants transaction: %w", err)
	}

	return nil
}

// GetVariantsByImageID lists all generated variants for a specific image
func (r *Repository) GetVariantsByImageID(ctx context.Context, imageID uuid.UUID) ([]domain.ImageVariant, error) {
	query := `
		SELECT id, image_id, size_type, format, path, width, height, file_size_bytes, created_at
		FROM image_variants
		WHERE image_id = $1
		ORDER BY created_at ASC;
	`

	rows, err := r.pool.Query(ctx, query, imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query variants: %w", err)
	}
	defer rows.Close()

	var variants []domain.ImageVariant
	for rows.Next() {
		var v domain.ImageVariant
		if err := rows.Scan(
			&v.ID,
			&v.ImageID,
			&v.SizeType,
			&v.Format,
			&v.Path,
			&v.Width,
			&v.Height,
			&v.FileSizeBytes,
			&v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan variant row: %w", err)
		}
		variants = append(variants, v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating variants: %w", err)
	}

	return variants, nil
}
