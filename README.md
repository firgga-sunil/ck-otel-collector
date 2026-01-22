# ck-otel-collector

This is a ck-otel-collector build that includes:

- **OTLP gRPC Receiver** - Receives telemetry data via OTLP protocol
- **Batch Processor** - Batches telemetry data for efficient processing
- **Custom Metrics Aggregator Processor** - Your custom processor for metrics aggregation
- **Prometheus Exporter** - Exports metrics to Prometheus

## Building

If you want to run it outside docker, kind, etc. and have `go` installed, then run
```sh
go install go.opentelemetry.io/collector/cmd/builder@v0.128.0
GOTOOLCHAIN=go1.23.12 make build-ck-intel-collector
./cmd/ck-otelcol --config example-config.yaml
```

### Prerequisites

The following tools need to be installed before building the ck-otel-collector:

#### 1. Install Go

Download and install Go 1.23 or later:

**macOS:**
```bash
# Using Homebrew
brew install go

# Or download from https://golang.org/dl/
```

**Linux:**
```bash
# Download and install Go
wget https://go.dev/dl/go1.23.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH=$PATH:/usr/local/go/bin
```

**Windows:**
Download the Windows installer from https://golang.org/dl/

Verify installation:
```bash
go version
```

#### 2. Install OpenTelemetry Collector Builder (OCB)

```bash
go install go.opentelemetry.io/collector/cmd/builder@latest

# The binary will be installed to $GOPATH/bin/builder or $HOME/go/bin/builder
# Make sure $GOPATH/bin or $HOME/go/bin is in your PATH
```

#### 3. Install Docker (Optional, for Docker builds)

**macOS:**
```bash
# Using Homebrew
brew install --cask docker

# Or download Docker Desktop from https://docker.com
```

**Linux:**
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install docker.io

# CentOS/RHEL
sudo yum install docker
```

**Windows:**
Download Docker Desktop from https://docker.com

#### 4. Install Kubernetes Tools (for deployment)

**kubectl:**
```bash
# macOS
brew install kubectl

# Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
```

**Helm:**
```bash
# macOS
brew install helm

# Linux
curl https://get.helm.sh/helm-v3.12.0-linux-amd64.tar.gz | tar xz
sudo mv linux-amd64/helm /usr/local/bin/
```

**Kind (for local development):**
```bash
# macOS
brew install kind

# Linux
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
```

### Build Commands

```bash
# Build the ck-intel-collector
make build-ck-intel-collector
# or
make all

# Clean build artifacts
make clean

# Run tests for all modules
make test

# Tidy up go modules
make tidy

# Show available components
make components

# Show version
make version

# Show all modules
make show-modules
```

## Docker

### How Docker Builds Work

The project uses **multi-stage Docker builds** similar to the OpenTelemetry Collector contrib repository:

1. **Multi-stage Docker builds** with the OpenTelemetry Collector Builder (OCB) tool
2. **Build the collector inside Docker** using the `builder` tool
3. **Copy the binary** to a minimal runtime image

This approach ensures:
- ✅ **Consistent builds** across different platforms
- ✅ **Minimal image size** (only the binary, no build tools)
- ✅ **Security** (no build tools in production image)

### Context-Aware Docker Builds

The project uses a **unified `dockerise` command** that automatically chooses the right Docker build strategy based on the context:

```bash
# For kind context (local development) - uses docker build
make dockerise CONTEXT=kind NAMESPACE=ckqa

# For AWS contexts (production) - uses docker buildx with multi-platform
make dockerise CONTEXT=aws-ck-qa NAMESPACE=demo
make dockerise CONTEXT=aws-ck-qa NAMESPACE=codekarma
make dockerise CONTEXT=aws-prod NAMESPACE=codekarma
```

### Docker Commands

```bash
# Context-aware Docker build (recommended)
make dockerise CONTEXT=kind NAMESPACE=ckqa

# Run in Docker
make docker-run
```

## Kubernetes Deployment

### Context and Namespace Mapping

The deployment system supports three main contexts with specific namespace mappings:

| **Context** | **Namespaces** | **Environment** | **Purpose** |
|-------------|----------------|-----------------|-------------|
| `kind` | `ckqa`, `demo`, `codekarma` | Local development | Testing and development |
| `aws-ck-qa` | `codekarma`, `demo` | AWS development environment | Development and testing |
| `aws-prod` | `codekarma` | AWS production | Production deployment |

### Local Development (Kind)

For local development and testing:

```bash
# Deploy to kind cluster in ckqa namespace
make build-and-deploy-kind-ckqa

# Deploy to kind cluster in demo namespace  
make build-and-deploy-kind-demo

# Generic deployment (requires NAMESPACE parameter)
make build-and-deploy-kind-to-namespace NAMESPACE=ckqa
```

### AWS Deployment

For production deployments to AWS clusters:

#### Prerequisites

1. **Registry Credentials**: Set up GitHub Package Registry credentials
   ```bash
   # Option 1: Use gradle.properties (recommended)
   make setup-gradle-props
   # Then edit ~/.gradle/gradle.properties with your GitHub credentials
   
   # Option 2: Environment variables
   export DOCKER_REGISTRY_USERNAME=your-github-username
   export DOCKER_REGISTRY_TOKEN=your-github-token
   ```

2. **Kubernetes Context**: Ensure your kubectl context is set to the AWS cluster
   ```bash
   # Verify context
   make verify-context CONTEXT=aws-ck-qa
   make verify-context CONTEXT=aws-prod
   ```

#### Deployment Commands

```bash
# Specific deployment targets (recommended)
make build-and-deploy-aws-demo      # Demo environment
make build-and-deploy-aws-dev       # Development environment  
make build-and-deploy-aws-prod      # Production environment

# Generic deployment (requires parameters)
make build-and-deploy-aws-to-namespace CONTEXT=aws-ck-qa NAMESPACE=demo
```

### Helm Values Files

The project includes environment-specific Helm values files:

- **Local (Kind)**: `helm/values.yaml`
- **AWS Demo**: `helm/values-aws-demo.yaml`  
- **AWS Development**: `helm/values-aws-dev.yaml`
- **AWS Production**: `helm/values-aws-prod.yaml`

### Key Ports in Kubernetes:
- `4319` - OTLP gRPC receiver
- `8888` - Prometheus metrics export
- `8887` - Internal telemetry metrics
- `13134` - Health check endpoint

## Configuration

The collector supports the following components:

### Receivers
- `otlp`: OTLP gRPC and HTTP receiver

### Processors
- `batch`: Batching processor
- `metricsaggregator`: Custom metrics aggregation processor

### Exporters
- `prometheus`: Prometheus metrics exporter

## Development Workflow

### Testing

```bash
# Run tests for all modules
make test

# Run tests and build
make test-and-build
```

### Module Management

```bash
# Show all modules
make show-modules

# Tidy all modules
make tidy
```

### Context Verification

```bash
# Verify kind context
make verify-context CONTEXT=kind

# Verify AWS CKQA context
make verify-context CONTEXT=aws-ck-qa

# Verify AWS production context
make verify-context CONTEXT=aws-prod
```

### Registry Management

```bash
# Check credentials
make check-credentials

# Setup gradle.properties
make setup-gradle-props

# Create secrets for AWS deployment
make create-secrets CONTEXT=aws-ck-qa NAMESPACE=demo
```

## Available Make Targets

### Build Targets
- `build-ck-intel-collector` - Build the ck-intel-collector binary
- `test` - Run tests for all modules
- `test-and-build` - Run tests and build the collector binary
- `clean` - Clean build artifacts
- `tidy` - Tidy up go modules for all modules

### Docker Targets
- `dockerise` - Context-aware Docker build (use CONTEXT and NAMESPACE parameters)
- `docker-run` - Run the collector in Docker

### Local Deployment (Kind)
- `build-and-deploy-kind-ckqa` - Build and deploy to kind cluster in ckqa namespace
- `build-and-deploy-kind-demo` - Build and deploy to kind cluster in demo namespace
- `build-and-deploy-kind-to-namespace` - Generic kind deployment (use NAMESPACE parameter)

### AWS Deployment
- `build-and-deploy-aws-demo` - Deploy to AWS demo cluster (aws-ck-qa context, demo namespace)
- `build-and-deploy-aws-dev` - Deploy to AWS development cluster (aws-ck-qa context, codekarma namespace)
- `build-and-deploy-aws-prod` - Deploy to AWS production cluster (aws-prod context, codekarma namespace)
- `build-and-deploy-aws-to-namespace` - Generic AWS deployment (use CONTEXT and NAMESPACE parameters)

### Generic Targets (Require Parameters)
- `verify-context` - Verify kubectl context (use CONTEXT parameter)
- `create-secrets` - Create/update registry secrets (use CONTEXT and NAMESPACE parameters)
- `dockerise` - Build and push Docker image (use CONTEXT and NAMESPACE parameters)

### Utilities
- `show-modules` - Show all modules that will be tested/tidied
- `check-helm-versions` - Check available OpenTelemetry Helm chart versions
- `components` - Show available components
- `version` - Show collector version
- `update-helm-repo` - Add/update OpenTelemetry Helm repository
- `help` - Display comprehensive help

## Examples

### Local Development
```bash
# Quick local development cycle
make build-and-deploy-kind-demo
```

### AWS Deployments
```bash
# Deploy to demo environment
make build-and-deploy-aws-demo

# Deploy to development environment  
make build-and-deploy-aws-dev

# Deploy to production environment
make build-and-deploy-aws-prod

# Generic deployment with parameters
make build-and-deploy-aws-to-namespace CONTEXT=aws-ck-qa NAMESPACE=demo
```

### Docker Builds
```bash
# Local development build
make dockerise CONTEXT=kind NAMESPACE=ckqa

# Production builds
make dockerise CONTEXT=aws-ck-qa NAMESPACE=demo
make dockerise CONTEXT=aws-ck-qa NAMESPACE=codekarma
make dockerise CONTEXT=aws-prod NAMESPACE=codekarma
```

### Override Credentials
```bash
export DOCKER_REGISTRY_USERNAME=your-username
export DOCKER_REGISTRY_TOKEN=your-token
make build-and-deploy-aws-demo
```

## Architecture

This ck-otel-collector is built using the OpenTelemetry Collector Builder with a minimal set of components for specific use cases, avoiding the overhead of including all contrib components. The deployment system supports both local development (Kind) and production (AWS) environments with environment-specific configurations.

## Support

For help with available targets:
```bash
make help
``` 