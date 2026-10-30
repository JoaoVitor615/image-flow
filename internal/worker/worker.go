package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/JoaoVitor615/image-flow/internal/processor"
	"github.com/JoaoVitor615/image-flow/internal/queue"
	"github.com/JoaoVitor615/image-flow/internal/storage"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// Repository defines database operations required by the worker
type Repository interface {
	GetJobByID(ctx context.Context, id uuid.UUID) (*domain.Job, error)
	UpdateJob(ctx context.Context, id uuid.UUID, status string, attempts int, errorMessage *string) error
	CreateVariants(ctx context.Context, variants []domain.ImageVariant) error
}

// VariantProcessor defines the contract for generating image variants
type VariantProcessor interface {
	ProcessVariants(r io.Reader) ([]processor.ProcessedVariant, error)
}

// ImageWorker orchestrates consumption of upload events, derivative generation, storage, and persistence
type ImageWorker struct {
	consumer    queue.EventConsumer
	repo        Repository
	storage     storage.Storage
	processor   VariantProcessor
	concurrency int
}

// NewImageWorker creates a new ImageWorker with all required dependencies and concurrency limit
func NewImageWorker(
	consumer queue.EventConsumer,
	repo Repository,
	storage storage.Storage,
	proc VariantProcessor,
	concurrency int,
) *ImageWorker {
	return &ImageWorker{
		consumer:    consumer,
		repo:        repo,
		storage:     storage,
		processor:   proc,
		concurrency: concurrency,
	}
}

// ProcessEvent executes the complete variant generation flow for a single image upload event
func (w *ImageWorker) ProcessEvent(ctx context.Context, event *domain.ImageUploadedEvent) error {
	job, err := w.repo.GetJobByID(ctx, event.JobID)
	if err != nil {
		return fmt.Errorf("failed to fetch job %s: %w", event.JobID, err)
	}

	job.Attempts++
	if err := w.repo.UpdateJob(ctx, job.ID, "processing", job.Attempts, nil); err != nil {
		return fmt.Errorf("failed to update job to processing: %w", err)
	}

	file, err := w.storage.Open(event.OriginalPath)
	if err != nil {
		w.handleFailure(ctx, job, fmt.Errorf("failed to open original file: %w", err))
		return fmt.Errorf("failed to open original image file %s: %w", event.OriginalPath, err)
	}
	defer file.Close()

	variants, err := w.processor.ProcessVariants(file)
	if err != nil {
		w.handleFailure(ctx, job, fmt.Errorf("failed to process image variants: %w", err))
		return fmt.Errorf("failed to generate variants: %w", err)
	}

	var domainVariants []domain.ImageVariant

	for _, v := range variants {
		filename := fmt.Sprintf("%s_%s.%s", event.ImageID.String(), v.SizeType, v.Format)

		variantPath, err := w.storage.SaveVariant(filename, bytes.NewReader(v.Data))
		if err != nil {
			w.handleFailure(ctx, job, fmt.Errorf("failed to save variant %s: %w", filename, err))
			return fmt.Errorf("failed to save variant file %s to storage: %w", filename, err)
		}

		domainVariants = append(domainVariants, domain.ImageVariant{
			ImageID:       event.ImageID,
			SizeType:      v.SizeType,
			Format:        v.Format,
			Path:          variantPath,
			Width:         v.Width,
			Height:        v.Height,
			FileSizeBytes: v.FileSizeBytes,
		})
	}

	if err := w.repo.CreateVariants(ctx, domainVariants); err != nil {
		w.handleFailure(ctx, job, fmt.Errorf("failed to persist variants in database: %w", err))
		return fmt.Errorf("failed to persist image variants in database: %w", err)
	}

	if err := w.repo.UpdateJob(ctx, job.ID, "completed", job.Attempts, nil); err != nil {
		return fmt.Errorf("failed to update job to completed: %w", err)
	}

	return nil
}

// handleFailure updates the job record with failure status or retains pending for retry if attempts remain
func (w *ImageWorker) handleFailure(ctx context.Context, job *domain.Job, processErr error) {
	errMsg := processErr.Error()
	status := "failed"
	if job.Attempts < job.MaxAttempts {
		status = "pending"
	}

	_ = w.repo.UpdateJob(ctx, job.ID, status, job.Attempts, &errMsg)
}

// Start begins the bounded concurrent event processing loop until the context is cancelled
func (w *ImageWorker) Start(ctx context.Context) error {
	log.Printf("Image worker started with max concurrency %d, listening for upload events...", w.concurrency)

	sem := make(chan struct{}, w.concurrency)
	var wg sync.WaitGroup

	for {
		// Acquire a semaphore slot before fetching next message (true backpressure)
		select {
		case <-ctx.Done():
			log.Println("Image worker received shutdown signal, waiting for active jobs to finish...")
			wg.Wait()
			log.Println("All active worker jobs completed, shutting down cleanly")
			return nil
		case sem <- struct{}{}:
		}

		//. Fetch next message from Kafka
		event, msg, err := w.consumer.FetchImageUploaded(ctx)
		if err != nil {
			<-sem // Release slot if fetch fails
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				log.Println("Context cancelled during message fetch, waiting for active jobs...")
				wg.Wait()
				return nil
			}
			log.Printf("Error fetching event from Kafka: %v", err)
			continue
		}

		// Dispatch processing in a bounded goroutine
		wg.Add(1)
		go func(e *domain.ImageUploadedEvent, m kafka.Message) {
			defer func() {
				<-sem // Release semaphore slot for next message
				wg.Done()
			}()

			if e == nil {
				log.Printf("Skipping unparseable message and committing offset")
				_ = w.consumer.Commit(ctx, m)
				return
			}

			log.Printf("Processing image derivatives for Job ID: %s, Image ID: %s", e.JobID, e.ImageID)
			if err := w.ProcessEvent(ctx, e); err != nil {
				log.Printf("Failed processing Job ID %s: %v", e.JobID, err)

				job, _ := w.repo.GetJobByID(ctx, e.JobID)
				if job != nil && job.Attempts >= job.MaxAttempts {
					_ = w.consumer.Commit(ctx, m)
				}
				return
			}

			if err := w.consumer.Commit(ctx, m); err != nil {
				log.Printf("Failed to commit offset for Job ID %s: %v", e.JobID, err)
			} else {
				log.Printf("Successfully generated derivatives and completed Job ID: %s", e.JobID)
			}
		}(event, msg)
	}
}
