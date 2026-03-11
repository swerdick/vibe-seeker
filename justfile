mod backend
mod frontend

# List all available recipes
default:
    @just --list

project_name := "vibe-seeker"
image_tag := env("IMAGE_TAG", "latest")
container_cli := env("CONTAINER_CLI", "podman")

# Start infrastructure (postgres) for local development
infra:
    {{container_cli}} compose up -d

# Start all services for local development
dev:
    {{container_cli}} compose --profile dev up

# Build all sub-projects
build:
    just backend::build
    just frontend::build

# Run unit tests
test:
    just backend::test
    just frontend::test

# Run integration tests
test-integration:
    just backend::test-integration
    just frontend::test-integration

# Run all static analysis
check:
    just backend::check
    just frontend::check

# Format code
fmt:
    just backend::fmt
    just frontend::fmt

# Remove build artifacts
clean:
    just backend::clean
    just frontend::clean

# Build container images
container-build:
    just backend::container-build
    just frontend::container-build

# Start all services in containers
up:
    {{container_cli}} compose --profile prod up --build -d

# Stop all services
down:
    {{container_cli}} compose --profile dev --profile prod down

# Full CI pipeline
ci: check test test-integration build container-build
