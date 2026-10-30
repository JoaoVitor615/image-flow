package worker_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/JoaoVitor615/image-flow/internal/processor"
	"github.com/JoaoVitor615/image-flow/internal/worker"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type mockRepository struct {
	job               *domain.Job
	getJobErr         error
	updateJobErr      error
	createVariantsErr error
	createdVariants   []domain.ImageVariant
	updatedStatus     string
	updatedAttempts   int
	updatedErrorMsg   *string
}

func (m *mockRepository) GetJobByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	if m.getJobErr != nil {
		return nil, m.getJobErr
	}
	return m.job, nil
}

func (m *mockRepository) UpdateJob(ctx context.Context, id uuid.UUID, status string, attempts int, errorMessage *string) error {
	if m.updateJobErr != nil {
		return m.updateJobErr
	}
	m.updatedStatus = status
	m.updatedAttempts = attempts
	m.updatedErrorMsg = errorMessage
	if m.job != nil {
		m.job.Status = status
		m.job.Attempts = attempts
		m.job.ErrorMessage = errorMessage
	}
	return nil
}

func (m *mockRepository) CreateVariants(ctx context.Context, variants []domain.ImageVariant) error {
	if m.createVariantsErr != nil {
		return m.createVariantsErr
	}
	m.createdVariants = append(m.createdVariants, variants...)
	return nil
}

type mockStorage struct {
	openReader     io.ReadCloser
	openErr        error
	saveVariantErr error
	savedVariants  map[string][]byte
}

func (m *mockStorage) SaveOriginal(filename string, r io.Reader) (string, error) {
	return "originals/" + filename, nil
}

func (m *mockStorage) SaveVariant(filename string, r io.Reader) (string, error) {
	if m.saveVariantErr != nil {
		return "", m.saveVariantErr
	}
	if m.savedVariants == nil {
		m.savedVariants = make(map[string][]byte)
	}
	data, _ := io.ReadAll(r)
	m.savedVariants[filename] = data
	return "variants/" + filename, nil
}

func (m *mockStorage) Open(path string) (io.ReadCloser, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	return m.openReader, nil
}

type mockProcessor struct {
	variants   []processor.ProcessedVariant
	processErr error
}

func (m *mockProcessor) ProcessVariants(r io.Reader) ([]processor.ProcessedVariant, error) {
	if m.processErr != nil {
		return nil, m.processErr
	}
	return m.variants, nil
}

type mockConsumer struct {
	eventsToFetch []*domain.ImageUploadedEvent
	fetchErr      error
	committedMsgs []kafka.Message
	commitErr     error
	fetchCallIdx  int
}

func (m *mockConsumer) FetchImageUploaded(ctx context.Context) (*domain.ImageUploadedEvent, kafka.Message, error) {
	if m.fetchErr != nil {
		return nil, kafka.Message{}, m.fetchErr
	}
	if m.fetchCallIdx < len(m.eventsToFetch) {
		event := m.eventsToFetch[m.fetchCallIdx]
		m.fetchCallIdx++
		return event, kafka.Message{Offset: int64(m.fetchCallIdx)}, nil
	}
	<-ctx.Done()
	return nil, kafka.Message{}, ctx.Err()
}

func (m *mockConsumer) Commit(ctx context.Context, msg kafka.Message) error {
	if m.commitErr != nil {
		return m.commitErr
	}
	m.committedMsgs = append(m.committedMsgs, msg)
	return nil
}

func (m *mockConsumer) Close() error {
	return nil
}

func TestImageWorker_ProcessEvent(t *testing.T) {
	ctx := context.Background()

	setupMocks := func() (*domain.Job, *domain.ImageUploadedEvent, *mockRepository, *mockStorage, *mockProcessor) {
		jobID := uuid.New()
		imageID := uuid.New()

		job := &domain.Job{
			ID:          jobID,
			ImageID:     imageID,
			Status:      "pending",
			Attempts:    0,
			MaxAttempts: 3,
		}

		event := &domain.ImageUploadedEvent{
			EventID:      uuid.New().String(),
			EventType:    "image.uploaded",
			Version:      "v1",
			ImageID:      imageID,
			JobID:        jobID,
			OriginalPath: "originals/sample.png",
			CreatedAt:    time.Now().UTC(),
		}

		repo := &mockRepository{job: job}
		storage := &mockStorage{
			openReader: io.NopCloser(bytes.NewReader([]byte("sample image bytes"))),
		}
		proc := &mockProcessor{
			variants: []processor.ProcessedVariant{
				{
					SizeType:      "thumbnail",
					Format:        "webp",
					Width:         200,
					Height:        150,
					Data:          []byte("thumb-webp-data"),
					FileSizeBytes: 15,
				},
				{
					SizeType:      "catalog",
					Format:        "jpeg",
					Width:         800,
					Height:        600,
					Data:          []byte("catalog-jpeg-data"),
					FileSizeBytes: 17,
				},
			},
		}

		return job, event, repo, storage, proc
	}

	t.Run("should successfully process all variants, persist them and mark job completed", func(t *testing.T) {
		_, event, repo, storage, proc := setupMocks()
		w := worker.NewImageWorker(nil, repo, storage, proc)

		err := w.ProcessEvent(ctx, event)
		if err != nil {
			t.Fatalf("expected successful process, got: %v", err)
		}

		if repo.updatedStatus != "completed" {
			t.Fatalf("expected job status 'completed', got: %s", repo.updatedStatus)
		}
		if repo.updatedAttempts != 1 {
			t.Fatalf("expected 1 attempt, got: %d", repo.updatedAttempts)
		}
		if len(repo.createdVariants) != 2 {
			t.Fatalf("expected 2 variants saved in repo, got: %d", len(repo.createdVariants))
		}
		if len(storage.savedVariants) != 2 {
			t.Fatalf("expected 2 variants saved in storage, got: %d", len(storage.savedVariants))
		}
	})

	t.Run("should fail if job cannot be retrieved from repository", func(t *testing.T) {
		_, event, repo, storage, proc := setupMocks()
		repo.getJobErr = errors.New("db error")
		w := worker.NewImageWorker(nil, repo, storage, proc)

		err := w.ProcessEvent(ctx, event)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("should fail and update status to pending when storage open fails and attempts < max", func(t *testing.T) {
		job, event, repo, storage, proc := setupMocks()
		storage.openErr = errors.New("file not found")
		w := worker.NewImageWorker(nil, repo, storage, proc)

		err := w.ProcessEvent(ctx, event)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if job.Status != "pending" {
			t.Fatalf("expected retry status 'pending', got: %s", job.Status)
		}
	})

	t.Run("should fail and update status to failed when processor fails and attempts reach max", func(t *testing.T) {
		job, event, repo, storage, proc := setupMocks()
		job.Attempts = 2 // will become 3 (max_attempts = 3)
		proc.processErr = errors.New("corrupt image data")
		w := worker.NewImageWorker(nil, repo, storage, proc)

		err := w.ProcessEvent(ctx, event)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if job.Status != "failed" {
			t.Fatalf("expected status 'failed', got: %s", job.Status)
		}
		if job.ErrorMessage == nil || *job.ErrorMessage != "failed to process image variants: corrupt image data" {
			t.Fatalf("unexpected error message: %v", job.ErrorMessage)
		}
	})

	t.Run("should fail when saving variant to storage fails", func(t *testing.T) {
		_, event, repo, storage, proc := setupMocks()
		storage.saveVariantErr = errors.New("disk full")
		w := worker.NewImageWorker(nil, repo, storage, proc)

		err := w.ProcessEvent(ctx, event)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("should fail when creating variants in repository fails", func(t *testing.T) {
		_, event, repo, storage, proc := setupMocks()
		repo.createVariantsErr = errors.New("db error")
		w := worker.NewImageWorker(nil, repo, storage, proc)

		err := w.ProcessEvent(ctx, event)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("should fail when updating job to completed fails", func(t *testing.T) {
		_, event, repo, storage, proc := setupMocks()
		repo.updateJobErr = errors.New("cannot complete job")
		w := worker.NewImageWorker(nil, repo, storage, proc)

		err := w.ProcessEvent(ctx, event)
		if err == nil {
			t.Fatal("expected error when completing job fails, got nil")
		}
	})
}

func TestImageWorker_Start(t *testing.T) {
	t.Run("should stop cleanly when context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		consumer := &mockConsumer{}
		w := worker.NewImageWorker(consumer, nil, nil, nil)

		err := w.Start(ctx)
		if err != nil {
			t.Fatalf("expected nil error on cancelled context, got: %v", err)
		}
	})

	t.Run("should process event and commit offset", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		jobID := uuid.New()
		imageID := uuid.New()

		job := &domain.Job{
			ID:          jobID,
			ImageID:     imageID,
			Status:      "pending",
			Attempts:    0,
			MaxAttempts: 3,
		}

		event := &domain.ImageUploadedEvent{
			EventID:      uuid.New().String(),
			EventType:    "image.uploaded",
			Version:      "v1",
			ImageID:      imageID,
			JobID:        jobID,
			OriginalPath: "originals/sample.png",
			CreatedAt:    time.Now().UTC(),
		}

		consumer := &mockConsumer{
			eventsToFetch: []*domain.ImageUploadedEvent{event},
		}
		repo := &mockRepository{job: job}
		storage := &mockStorage{
			openReader: io.NopCloser(bytes.NewReader([]byte("sample image bytes"))),
		}
		proc := &mockProcessor{
			variants: []processor.ProcessedVariant{
				{SizeType: "thumbnail", Format: "webp", Width: 200, Height: 150, Data: []byte("d")},
			},
		}

		w := worker.NewImageWorker(consumer, repo, storage, proc)

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := w.Start(ctx)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		if len(consumer.committedMsgs) != 1 {
			t.Fatalf("expected 1 committed message, got: %d", len(consumer.committedMsgs))
		}
	})

	t.Run("should skip and commit malformed nil event to avoid blocking queue", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		consumer := &mockConsumer{
			eventsToFetch: []*domain.ImageUploadedEvent{nil},
		}

		w := worker.NewImageWorker(consumer, nil, nil, nil)

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := w.Start(ctx)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		if len(consumer.committedMsgs) != 1 {
			t.Fatalf("expected malformed message to be committed, got: %d", len(consumer.committedMsgs))
		}
	})

	t.Run("should commit message when processing fails and max attempts reached", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		jobID := uuid.New()
		imageID := uuid.New()

		job := &domain.Job{
			ID:          jobID,
			ImageID:     imageID,
			Status:      "pending",
			Attempts:    2,
			MaxAttempts: 3,
		}

		event := &domain.ImageUploadedEvent{
			EventID:      uuid.New().String(),
			ImageID:      imageID,
			JobID:        jobID,
			OriginalPath: "originals/corrupt.png",
		}

		consumer := &mockConsumer{
			eventsToFetch: []*domain.ImageUploadedEvent{event},
		}
		repo := &mockRepository{job: job}
		storage := &mockStorage{
			openReader: io.NopCloser(bytes.NewReader([]byte("bytes"))),
		}
		proc := &mockProcessor{
			processErr: errors.New("unrecoverable corrupt file"),
		}

		w := worker.NewImageWorker(consumer, repo, storage, proc)

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := w.Start(ctx)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		if len(consumer.committedMsgs) != 1 {
			t.Fatalf("expected poisoned message to be committed after max attempts, got: %d", len(consumer.committedMsgs))
		}
	})
}
