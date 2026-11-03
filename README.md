<p align="center">
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go" alt="Go Version"></a>
  <a href="https://www.postgresql.org/"><img src="https://img.shields.io/badge/PostgreSQL-16-336791?style=flat&logo=postgresql" alt="PostgreSQL"></a>
  <a href="https://kafka.apache.org/"><img src="https://img.shields.io/badge/Apache%20Kafka-KRaft-231F20?style=flat&logo=apachekafka" alt="Apache Kafka"></a>
  <a href="https://github.com/JoaoVitor615/image-flow/actions"><img src="https://github.com/JoaoVitor615/image-flow/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/JoaoVitor615/image-flow"><img src="https://img.shields.io/badge/Coverage-%E2%89%A575%25-brightgreen" alt="Coverage Threshold"></a>
</p>

<h1 align="center">⚡ ImageFlow</h1>

<p align="center">
  <strong>An asynchronous, event-driven image processing engine built with Go, Apache Kafka (KRaft), and PostgreSQL.</strong>
</p>

---

## 📌 Overview

**ImageFlow** is a distributed image processing system designed to handle heavy media workflows asynchronously without blocking HTTP client requests. 

When a client uploads an image, the API performs instant validations (magic numbers, dimension constraints, file limits), stores the original payload, records a processing job, and publishes an event to Apache Kafka. A decoupled background worker service consumes events under controlled concurrency, resizes the image preserving aspect ratio, encodes 6 distinct derivatives (thumbnail and catalog variants in WebP, JPEG, and PNG), and safely commits Kafka offsets upon completion.

---

## 🏛️ Architecture & Data Flow

```mermaid
flowchart TD
    Client(["🌐 Client / Frontend"])

    subgraph API_Service ["API Service (cmd/api)"]
        Router["chi HTTP Router"]
        Validator["Validation Pipeline"]
        API_Store["Storage Handler"]
    end

    subgraph Messaging ["Event Broker"]
        Kafka[("Apache Kafka KRaft")]
    end

    subgraph Storage_Layer ["Persistence and Storage"]
        PG[("PostgreSQL 16")]
        Disk[("Persistent Disk Storage")]
    end

    subgraph Worker_Service ["Worker Service (cmd/worker)"]
        Consumer["Kafka Consumer Group"]
        Semaphore["Bounded Concurrency Semaphore"]
        Processor["Image Processor Engine"]
    end

    %% Flow connections
    Client -->|"1. POST /images (multipart)"| Router
    Router --> Validator
    Validator --> API_Store
    API_Store -->|"2. Save original"| Disk
    API_Store -->|"3. Record image and pending job"| PG
    API_Store -->|"4. Publish event"| Kafka
    Router -.->|"5. 202 Accepted (job_id, image_id)"| Client

    Kafka -->|"6. Fetch event"| Consumer
    Consumer --> Semaphore
    Semaphore --> Processor
    Processor -->|"7. Save 6 derivatives"| Disk
    Processor -->|"8. Insert variants and complete job"| PG
    Processor -->|"9. Commit offset"| Kafka

    Client -->|"10. GET /images/:id with filters"| Router
    Router -->|"Serve pre-generated asset"| Disk
```

---

## 🛠️ Tech Stack

| Component | Technology | Rationale |
| :--- | :--- | :--- |
| **Language** | Go 1.24+ | High performance, concurrency primitives (channels/goroutines), low memory footprint |
| **Router** | [`go-chi/chi/v5`](https://github.com/go-chi/chi) | Idiomatic, zero-allocation HTTP routing with middleware chaining |
| **Database Driver** | [`jackc/pgx/v5`](https://github.com/jackc/pgx) | High-performance PostgreSQL driver with native connection pooling (`pgxpool`) |
| **Message Broker** | Apache Kafka (KRaft mode) | Enterprise-grade distributed event streaming without ZooKeeper dependency |
| **Kafka Client** | [`segmentio/kafka-go`](https://github.com/segmentio/kafka-go) | Pure Go implementation with explicit offset management and context support |
| **Database** | PostgreSQL 16 | Relational consistency, UUID primary keys (`pgcrypto`), and structured state tracking |
| **DevOps / CI** | Docker Compose & GitHub Actions | Automated build, test, and minimum 75% coverage enforcement on PRs |

---

## 📂 Derivative Generation Specs

For each successfully uploaded image, the background worker automatically generates **6 derivatives**:

| Size Type | Max Dimensions | Aspect Ratio | Target Formats |
| :--- | :--- | :--- | :--- |
| **`thumbnail`** | 200 × 200 px | Preserved | WebP, JPEG, PNG |
| **`catalog`** | 800 px (width) | Preserved | WebP, JPEG, PNG |

---

## 🚀 Getting Started

### Prerequisites

- [Go 1.24+](https://golang.org/dl/)
- [Docker](https://www.docker.com/) & Docker Compose

### 1. Clone the repository

```bash
git clone https://github.com/JoaoVitor615/image-flow.git
cd image-flow
```

### 2. Start Infrastructure Services

Spin up PostgreSQL, Apache Kafka (KRaft), and Kafka UI:

```bash
docker compose up -d
```

- **PostgreSQL:** `localhost:5432`
- **Apache Kafka:** `localhost:9092`
- **Kafka UI:** [http://localhost:8085](http://localhost:8085)

### 3. Apply Database Migrations

```bash
# Execute schema migration against the local database
psql -h localhost -U postgres -d imageflow -f migrations/000001_init_schema.up.sql
```

### 4. Run the API and Worker

In separate terminal tabs:

```bash
# Terminal 1: Run HTTP API
go run cmd/api/main.go

# Terminal 2: Run Background Worker
go run cmd/worker/main.go
```

---

## 📡 API Reference

### 1. Upload an Image
Accepts a multipart file upload (`image`), validates dimensions and format, and queues processing.

- **URL:** `POST /images`
- **Content-Type:** `multipart/form-data`
- **Body:** `image` (binary file: JPEG, PNG, or WebP; up to 10MB)

```bash
curl -X POST http://localhost:8080/images \
  -F "image=@/path/to/sample.jpg"
```

**Response (`202 Accepted`):**
```json
{
  "image_id": "423f71c4-118f-4f27-992a-3dd7c0eb83ad",
  "job_id": "993a4bc1-c741-4c6e-8273-0ff7d5c95c80",
  "status": "pending"
}
```

---

### 2. Retrieve an Image or Variant
Fetches either the original image or a pre-generated derivative.

- **URL:** `GET /images/{id}`
- **Query Parameters:**
  - `size` *(optional)*: `thumbnail` | `catalog`
  - `format` *(optional)*: `webp` | `jpeg` | `png`

```bash
# Retrieve original image
curl -O http://localhost:8080/images/423f71c4-118f-4f27-992a-3dd7c0eb83ad

# Retrieve optimized WebP thumbnail
curl -O "http://localhost:8080/images/423f71c4-118f-4f27-992a-3dd7c0eb83ad?size=thumbnail&format=webp"

# Retrieve catalog size in PNG
curl -O "http://localhost:8080/images/423f71c4-118f-4f27-992a-3dd7c0eb83ad?size=catalog&format=png"
```

---

## ⚙️ Environment Variables

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PORT` | HTTP server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://postgres:postgres@localhost:5432/imageflow?sslmode=disable` |
| `STORAGE_BASE_PATH`| File system path for media storage | `./data/images` |
| `KAFKA_BROKERS` | Comma-separated Kafka bootstrap servers | `localhost:9092` |
| `KAFKA_TOPIC` | Kafka topic for upload events | `image-uploaded` |
| `KAFKA_GROUP_ID` | Consumer group ID for the worker | `image-processing-group` |
| `WORKER_CONCURRENCY`| Max concurrent goroutines per worker | `5` |
