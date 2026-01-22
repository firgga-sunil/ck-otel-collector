# Variables
BUILDER_CONFIG=builder-config.yaml
BUILDER=$(shell command -v builder 2>/dev/null || echo $(HOME)/go/bin/builder)
DIST_DIR=./cmd
BINARY_NAME=ck-intel-collector
DOCKER_IMAGE_NAME=ck-intel-collector
DOCKER_IMAGE_TAG=latest

# Registry configuration for AWS deployments
DOCKER_REGISTRY=ghcr.io
GRADLE_PROPS_FILE=$(HOME)/.gradle/gradle.properties
DOCKER_REGISTRY_USERNAME=$(shell echo $${DOCKER_REGISTRY_USERNAME:-$(shell grep '^gpr\.user=' $(GRADLE_PROPS_FILE) 2>/dev/null | cut -d'=' -f2 || echo "")})
DOCKER_REGISTRY_TOKEN=$(shell echo $${DOCKER_REGISTRY_TOKEN:-$(shell grep '^gpr\.token=' $(GRADLE_PROPS_FILE) 2>/dev/null | cut -d'=' -f2 || echo "")})

# Helm chart versions (pin to specific versions for stability)
OTEL_HELM_CHART_VERSION=0.126.0

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Define modules (just paths - names auto-extracted from last path segment)
MODULES := processor/metricsaggregatorprocessor \
           exporter/prometheusexporter \
           internal/common/testdata \
           internal/coreinternal/testutil \
           cmd

.PHONY: all build clean test test-and-build build-ck-intel-collector docker-run ensure-kind-context update-helm-repo load-kind build-and-deploy-kind-ckqa build-and-deploy-kind-demo build-and-deploy-kind-codekarma build-and-deploy-aws-to-namespace build-and-deploy-aws-demo build-and-deploy-aws-ckqa build-and-deploy-aws-prod build-and-deploy-gcp-ckqa build-and-deploy-gcp-demo build-and-deploy-gcp-flipkart deploy-gcp-flipkart build-cleartrip verify-context create-secrets dockerise check-credentials setup-gradle-props show-modules check-helm-versions components version help

all: build-ck-intel-collector

# Parameter validation functions
check-namespace: ## Check if NAMESPACE parameter is provided
	@if [ -z "$(NAMESPACE)" ]; then \
		echo "ERROR: NAMESPACE parameter is required!"; \
		echo "Usage: make <target> NAMESPACE=<namespace>"; \
		echo "Valid namespaces: demo, ckqa, codekarma"; \
		exit 1; \
	fi

check-context: ## Check if CONTEXT parameter is provided
	@if [ -z "$(CONTEXT)" ]; then \
		echo "ERROR: CONTEXT parameter is required!"; \
		echo "Usage: make <target> CONTEXT=<context>"; \
		echo "Valid contexts: kind, aws-ck-qa, aws-prod, gcp-ck-qa, gcp-demo, cleartrip, gcp-flipkart"; \
		exit 1; \
	fi

# Context verification function (generic)
verify-context: check-context ## Verify kubectl context (use CONTEXT parameter)
	@echo "Checking kubectl context for $(CONTEXT) deployment..."
	@CURRENT_CONTEXT=$$(kubectl config current-context); \
	case "$(CONTEXT)" in \
		"kind") \
			if [[ "$$CURRENT_CONTEXT" != *"kind"* ]]; then \
				echo "ERROR: Current kubectl context is '$$CURRENT_CONTEXT', not a kind context."; \
				echo "Please switch to a kind context before deploying locally."; \
				echo "Available contexts:"; \
				kubectl config get-contexts; \
				exit 1; \
			fi; \
			echo "✓ Current context is kind: $$CURRENT_CONTEXT";; \
		"aws-ck-qa") \
			if [[ "$$CURRENT_CONTEXT" != *"aws"*"ck-qa"* ]]; then \
				echo "ERROR: Current kubectl context is '$$CURRENT_CONTEXT', not an AWS CKQA context."; \
				echo "Expected: AWS CKQA cluster context (should contain 'aws' and 'ck-qa')"; \
				echo "Available contexts:"; \
				kubectl config get-contexts; \
				echo "To switch context, run:"; \
				echo "  kubectl config use-context <aws-ckqa-context-name>"; \
				exit 1; \
			fi; \
			echo "✓ Current context is AWS CKQA: $$CURRENT_CONTEXT";; \
		"aws-prod") \
			if [[ "$$CURRENT_CONTEXT" != *"aws-prod"* ]]; then \
				echo "ERROR: Current kubectl context is '$$CURRENT_CONTEXT', not an AWS production context."; \
				echo "Expected: AWS production cluster context (should contain 'aws-prod')"; \
				echo "Available contexts:"; \
				kubectl config get-contexts; \
				echo "To switch context, run:"; \
				echo "  kubectl config use-context <aws-prod-context-name>"; \
				exit 1; \
			fi; \
			echo "✓ Current context is AWS Production: $$CURRENT_CONTEXT";; \
		"gcp-ck-qa") \
			if [[ "$$CURRENT_CONTEXT" != *"gke"* ]] && [[ "$$CURRENT_CONTEXT" != *"gcp"* ]]; then \
				echo "ERROR: Current kubectl context is '$$CURRENT_CONTEXT', not a GCP context."; \
				echo "Expected: GCP/GKE cluster context (should contain 'gke' or 'gcp')"; \
				echo "Available contexts:"; \
				kubectl config get-contexts; \
				echo "To switch context, run:"; \
				echo "  kubectl config use-context <gke-context-name>"; \
				exit 1; \
			fi; \
			echo "✓ Current context is GCP: $$CURRENT_CONTEXT";; \
		"gcp-demo") \
			if [[ "$$CURRENT_CONTEXT" != "gke_resounding-node-471205-f9_us-east1_demo-cluster" ]]; then \
				echo "ERROR: Current kubectl context is '$$CURRENT_CONTEXT', not a GCP context."; \
				echo "Expected: GCP/GKE cluster context (gke_resounding-node-471205-f9_us-east1_demo-cluster)"; \
				echo "Available contexts:"; \
				kubectl config get-contexts; \
				echo "To switch context, run:"; \
				echo "  kubectl config use-context <gke-context-name>"; \
				exit 1; \
			fi; \
			echo "✓ Current context is GCP: $$CURRENT_CONTEXT";; \
		"cleartrip") \
			echo "Skipping validation for Cleartrip: $$CURRENT_CONTEXT";; \
		"gcp-flipkart") \
			if [[ "$$CURRENT_CONTEXT" != "gke_fk-code-karma_asia-south1_gke-code-karma-prod-1" ]]; then \
				echo "ERROR: Current kubectl context is '$$CURRENT_CONTEXT', not the Flipkart context."; \
				echo "Expected: gke_fk-code-karma_asia-south1_gke-code-karma-prod-1"; \
				echo "Available contexts:"; \
				kubectl config get-contexts; \
				echo "To switch context, run:"; \
				echo "  kubectl config use-context gke_fk-code-karma_asia-south1_gke-code-karma-prod-1"; \
				exit 1; \
			fi; \
			echo "✓ Current context is Flipkart: $$CURRENT_CONTEXT";; \
		*) \
			echo "ERROR: Invalid CONTEXT parameter: $(CONTEXT)"; \
			echo "Valid contexts: kind, aws-ck-qa, aws-prod, gcp-ck-qa, gcp-demo, cleartrip, gcp-flipkart"; \
			exit 1;; \
	esac

# Validate namespace based on context
validate-namespace: check-namespace ## Validate namespace is appropriate for the given context
	@case "$(CONTEXT)-$(NAMESPACE)" in \
		"kind-ckqa"|"kind-demo"|"kind-codekarma") \
			echo "✓ Valid namespace $(NAMESPACE) for $(CONTEXT) context";; \
		"aws-ck-qa-codekarma"|"aws-ck-qa-demo"|"aws-ck-qa-ckqa") \
			echo "✓ Valid namespace $(NAMESPACE) for $(CONTEXT) context";; \
		"aws-prod-codekarma") \
			echo "✓ Valid namespace $(NAMESPACE) for $(CONTEXT) context";; \
		"gcp-ck-qa-codekarma") \
			echo "✓ Valid namespace $(NAMESPACE) for $(CONTEXT) context";; \
		"gcp-demo-codekarma") \
			echo "✓ Valid namespace $(NAMESPACE) for $(CONTEXT) context";; \
		"cleartrip-codekarma") \
			echo "✓ Valid namespace $(NAMESPACE) for $(CONTEXT) context";; \
		"gcp-flipkart-codekarma") \
			echo "✓ Valid namespace $(NAMESPACE) for $(CONTEXT) context";; \
		*) \
			echo "ERROR: Invalid CONTEXT-NAMESPACE combination: $(CONTEXT)-$(NAMESPACE)"; \
			echo "Valid combinations:"; \
			echo "  kind + ckqa/demo/codekarma"; \
			echo "  aws-ck-qa + codekarma/demo/ckqa"; \
			echo "  aws-prod + codekarma"; \
			echo "  gcp-ck-qa + codekarma"; \
			echo "  gcp-demo + codekarma"; \
			echo "  cleartrip + codekarma"; \
			echo "  gcp-flipkart + codekarma"; \
			exit 1;; \
	esac

# Secret creation function (generic)
create-secrets: verify-context validate-namespace check-credentials ## Create/update registry secrets (use CONTEXT and NAMESPACE parameters)
	@echo "Creating/updating registry secrets in $(CONTEXT) cluster (namespace: $(NAMESPACE))..."
	@echo "Using registry username: $(DOCKER_REGISTRY_USERNAME)"
	@kubectl create secret docker-registry ck-registry-secret \
		--docker-server=$(DOCKER_REGISTRY) \
		--docker-username="$(DOCKER_REGISTRY_USERNAME)" \
		--docker-password="$(DOCKER_REGISTRY_TOKEN)" \
		--namespace=$(NAMESPACE) \
		--save-config --dry-run=client -o yaml | kubectl apply -f -
	@echo "Registry secrets created/updated successfully"

check-credentials: ## Verify that registry credentials are available
	@echo "Checking registry credentials..."
	@if [ -z "$(DOCKER_REGISTRY_USERNAME)" ] || [ -z "$(DOCKER_REGISTRY_TOKEN)" ]; then \
		echo "ERROR: Registry credentials not found!"; \
		echo ""; \
		echo "Credentials are loaded from (in order of preference):"; \
		echo "  1. Environment variables: DOCKER_REGISTRY_USERNAME, DOCKER_REGISTRY_TOKEN"; \
		echo "  2. Gradle properties file: $(GRADLE_PROPS_FILE)"; \
		echo ""; \
		echo "To set up gradle.properties, run: make setup-gradle-props"; \
		echo "Or set environment variables:"; \
		echo "  export DOCKER_REGISTRY_USERNAME=your-username"; \
		echo "  export DOCKER_REGISTRY_TOKEN=your-token"; \
		echo ""; \
		if [ ! -f "$(GRADLE_PROPS_FILE)" ]; then \
			echo "Gradle properties file not found: $(GRADLE_PROPS_FILE)"; \
		else \
			echo "Gradle properties file exists but missing gpr.user or gpr.token"; \
		fi; \
		exit 1; \
	fi
	@echo "✓ Registry credentials found: $(DOCKER_REGISTRY_USERNAME)"

setup-gradle-props: ## Help set up ~/.gradle/gradle.properties file
	@echo "Setting up gradle.properties file..."
	@if [ ! -d "$(HOME)/.gradle" ]; then \
		echo "Creating ~/.gradle directory..."; \
		mkdir -p "$(HOME)/.gradle"; \
	fi
	@if [ -f "$(GRADLE_PROPS_FILE)" ]; then \
		echo "Gradle properties file already exists: $(GRADLE_PROPS_FILE)"; \
		echo "Current gpr settings:"; \
		grep '^gpr\.' "$(GRADLE_PROPS_FILE)" 2>/dev/null || echo "  No gpr.* properties found"; \
	else \
		echo "Creating new gradle.properties file..."; \
		echo "# GitHub Package Registry credentials" > "$(GRADLE_PROPS_FILE)"; \
		echo "# Replace with your actual GitHub username and personal access token" >> "$(GRADLE_PROPS_FILE)"; \
		echo "gpr.user=your-github-username" >> "$(GRADLE_PROPS_FILE)"; \
		echo "gpr.token=your-github-token" >> "$(GRADLE_PROPS_FILE)"; \
		echo "✓ Created $(GRADLE_PROPS_FILE)"; \
	fi
	@echo ""
	@echo "Please edit $(GRADLE_PROPS_FILE) and set:"
	@echo "  gpr.user=your-github-username"
	@echo "  gpr.token=your-github-personal-access-token"

build-ck-intel-collector: ## Build the ck-intel-collector
	@echo "Building ck-intel-collector..."
	$(BUILDER) --config=$(BUILDER_CONFIG)
	@echo "Build complete. Binary available at: $(DIST_DIR)/$(BINARY_NAME)"

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(DIST_DIR)
	@echo "Clean complete."

test: tidy ## Run tests for all modules
	@echo "Running tests for all modules..."
	@echo ""
	@FAILED_MODULES=""; \
	PASSED_MODULES=""; \
	for module_path in $(MODULES); do \
		module_name=$${module_path##*/}; \
		echo "Testing $$module_path..."; \
		if cd $$module_path && $(GOTEST) -v ./...; then \
			PASSED_MODULES="$$PASSED_MODULES $$module_name"; \
		else \
			FAILED_MODULES="$$FAILED_MODULES $$module_name"; \
		fi; \
		cd $(shell pwd); \
	done; \
	echo ""; \
	echo "=== TEST SUMMARY ==="; \
	if [ -n "$$PASSED_MODULES" ]; then \
		echo "✅ PASSED:$$PASSED_MODULES"; \
	fi; \
	if [ -n "$$FAILED_MODULES" ]; then \
		echo "❌ FAILED:$$FAILED_MODULES"; \
		exit 1; \
	else \
		echo "🎉 All tests passed!"; \
	fi

test-and-build: test build-ck-intel-collector ## Run tests and build the collector binary

tidy: ## Tidy up go modules for all modules
	@echo "Tidying up go modules for all modules..."
	@for module_path in $(MODULES); do \
		echo "Tidying $$module_path..."; \
		cd $$module_path && $(GOMOD) tidy; \
		cd $(shell pwd); \
	done
	@echo "All modules tidied successfully!"

docker-run: ## Run the collector in Docker
	@echo "Running collector in Docker..."
	docker run --rm -p 4317:4317 -p 4318:4318 -p 8889:8889 $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)

# Docker build and push function (context-aware)
dockerise: verify-context validate-namespace check-credentials ## Build and push Docker image (use CONTEXT and NAMESPACE parameters)
	@echo "Building Docker image for $(CONTEXT) context in $(NAMESPACE) namespace..."
	@if [ "$(CONTEXT)" = "kind" ]; then \
		echo "Using docker build for kind context (local development)..."; \
		docker build -f Dockerfile.multiarch -t $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) .; \
		echo "Docker image built for kind context: $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)"; \
	elif [ "$(CONTEXT)" = "aws-ck-qa" ] || [ "$(CONTEXT)" = "aws-prod" ]; then \
		echo "Using docker buildx for $(CONTEXT) context (production deployment)..."; \
		echo "Using registry username: $(DOCKER_REGISTRY_USERNAME)"; \
		echo "Logging into $(DOCKER_REGISTRY)..."; \
		echo "$(DOCKER_REGISTRY_TOKEN)" | docker login $(DOCKER_REGISTRY) -u "$(DOCKER_REGISTRY_USERNAME)" --password-stdin; \
		echo "Building multi-platform Docker image..."; \
		if [ "$(CONTEXT)" = "aws-ck-qa" ] && [ "$(NAMESPACE)" = "demo" ]; then ENV_NAME=demo; \
		elif [ "$(CONTEXT)" = "aws-ck-qa" ] && [ "$(NAMESPACE)" = "codekarma" ]; then ENV_NAME=ckqa; \
		elif [ "$(CONTEXT)" = "aws-ck-qa" ] && [ "$(NAMESPACE)" = "ckqa" ]; then ENV_NAME=ckqa; \
		elif [ "$(CONTEXT)" = "aws-prod" ] && [ "$(NAMESPACE)" = "codekarma" ]; then ENV_NAME=prod; \
		else ENV_NAME=unknown; fi; \
		docker buildx build --platform linux/amd64,linux/arm64 \
			-t $(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
			--push -f Dockerfile.multiarch .; \
		echo "Docker image built and pushed for $(CONTEXT) context"; \
	elif [ "$(CONTEXT)" = "gcp-ck-qa" ]; then \
		echo "Using docker buildx for $(CONTEXT) context (GCP deployment)..."; \
		echo "Using registry username: $(DOCKER_REGISTRY_USERNAME)"; \
		echo "Logging into $(DOCKER_REGISTRY)..."; \
		echo "$(DOCKER_REGISTRY_TOKEN)" | docker login $(DOCKER_REGISTRY) -u "$(DOCKER_REGISTRY_USERNAME)" --password-stdin; \
		echo "Building multi-platform Docker image..."; \
		ENV_NAME=gcp-ckqa; \
		docker buildx build --platform linux/amd64,linux/arm64 \
			-t $(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
			--push -f Dockerfile.multiarch .; \
		echo "Docker image built and pushed for $(CONTEXT) context"; \
	elif [ "$(CONTEXT)" = "gcp-demo" ]; then \
		echo "Using docker buildx for $(CONTEXT) context (GCP demo deployment)..."; \
		echo "Using registry username: $(DOCKER_REGISTRY_USERNAME)"; \
		echo "Logging into $(DOCKER_REGISTRY)..."; \
		echo "$(DOCKER_REGISTRY_TOKEN)" | docker login $(DOCKER_REGISTRY) -u "$(DOCKER_REGISTRY_USERNAME)" --password-stdin; \
		echo "Building multi-platform Docker image..."; \
		ENV_NAME=gcp-demo; \
		docker buildx build --platform linux/amd64,linux/arm64 \
			-t $(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
			--push -f Dockerfile.multiarch .; \
		echo "Docker image built and pushed for $(CONTEXT) context"; \
	elif [ "$(CONTEXT)" = "cleartrip" ]; then \
		echo "Using docker buildx for $(CONTEXT) context (Cleartrip deployment)..."; \
		echo "Using registry username: $(DOCKER_REGISTRY_USERNAME)"; \
		echo "Logging into $(DOCKER_REGISTRY)..."; \
		echo "$(DOCKER_REGISTRY_TOKEN)" | docker login $(DOCKER_REGISTRY) -u "$(DOCKER_REGISTRY_USERNAME)" --password-stdin; \
		echo "Building multi-platform Docker image..."; \
		ENV_NAME=cleartrip; \
		docker buildx build --platform linux/amd64,linux/arm64 \
			-t $(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
			--push -f Dockerfile.multiarch .; \
		echo "Docker image built and pushed for $(CONTEXT) context"; \
	elif [ "$(CONTEXT)" = "gcp-flipkart" ]; then \
		echo "Using docker buildx for $(CONTEXT) context (Flipkart deployment)..."; \
		echo "Using registry username: $(DOCKER_REGISTRY_USERNAME)"; \
		echo "Logging into $(DOCKER_REGISTRY)..."; \
		echo "$(DOCKER_REGISTRY_TOKEN)" | docker login $(DOCKER_REGISTRY) -u "$(DOCKER_REGISTRY_USERNAME)" --password-stdin; \
		echo "Building multi-platform Docker image..."; \
		ENV_NAME=flipkart; \
		docker buildx build --platform linux/amd64,linux/arm64 \
			-t $(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
			--push -f Dockerfile.multiarch .; \
		echo "Docker image built and pushed for $(CONTEXT) context"; \
	else \
		echo "ERROR: Invalid CONTEXT parameter: $(CONTEXT)"; \
		echo "Valid contexts: kind, aws-ck-qa, aws-prod, gcp-ck-qa, gcp-demo, cleartrip, gcp-flipkart"; \
		exit 1; \
	fi

# Generic AWS deployment target
build-and-deploy-aws-to-namespace: verify-context validate-namespace update-helm-repo ## Deploy to AWS cluster (use CONTEXT and NAMESPACE parameters)
	@echo "Deploying to $(CONTEXT) cluster in $(NAMESPACE) namespace..."
	@$(MAKE) create-secrets CONTEXT=$(CONTEXT) NAMESPACE=$(NAMESPACE)
	@$(MAKE) dockerise CONTEXT=$(CONTEXT) NAMESPACE=$(NAMESPACE)
	@if [ "$(NAMESPACE)" != "default" ]; then \
		echo "Ensuring $(NAMESPACE) namespace exists..."; \
		kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -; \
	fi
	@echo "Checking if ck-intel-collector release exists in $(NAMESPACE) namespace..."
	@if helm list -n $(NAMESPACE) | grep -q ck-intel-collector; then \
		echo "Uninstalling existing ck-intel-collector release..."; \
		helm uninstall ck-intel-collector -n $(NAMESPACE); \
	else \
		echo "No existing release found, proceeding with fresh installation..."; \
	fi
	@sleep 4
	@echo "Installing ck-intel-collector using Helm in $(NAMESPACE) namespace..."
	@# Determine environment name and values file based on context and namespace
	@if [ "$(CONTEXT)" = "aws-ck-qa" ] && [ "$(NAMESPACE)" = "demo" ]; then \
		ENV_NAME=demo; \
	elif [ "$(CONTEXT)" = "aws-ck-qa" ] && [ "$(NAMESPACE)" = "codekarma" ]; then \
		ENV_NAME=ckqa; \
	elif [ "$(CONTEXT)" = "aws-ck-qa" ] && [ "$(NAMESPACE)" = "ckqa" ]; then \
		ENV_NAME=ckqa; \
	elif [ "$(CONTEXT)" = "aws-prod" ] && [ "$(NAMESPACE)" = "codekarma" ]; then \
		ENV_NAME=prod; \
	else \
		ENV_NAME=unknown; \
	fi; \
	echo "Using custom image: $(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)"; \
	echo "Using values file: helm/values-aws-$$ENV_NAME.yaml"; \
	helm install ck-intel-collector open-telemetry/opentelemetry-collector \
		--version $(OTEL_HELM_CHART_VERSION) \
		-n $(NAMESPACE) \
		-f helm/values-aws-$$ENV_NAME.yaml \
		--set image.repository="$(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME)" \
		--set image.tag="$(DOCKER_IMAGE_TAG)" \
		--set image.pullPolicy=Always \
		--set imagePullSecrets[0].name="ck-registry-secret"
	@echo "AWS deployment completed successfully"

# Generic GCP deployment target
build-and-deploy-gcp-to-namespace: verify-context validate-namespace update-helm-repo ## Deploy to GCP cluster (use CONTEXT and NAMESPACE parameters)
	@echo "Deploying to $(CONTEXT) cluster in $(NAMESPACE) namespace..."
	@$(MAKE) create-secrets CONTEXT=$(CONTEXT) NAMESPACE=$(NAMESPACE)
	 @$(MAKE) dockerise CONTEXT=$(CONTEXT) NAMESPACE=$(NAMESPACE)
	@if [ "$(NAMESPACE)" != "default" ]; then \
		echo "Ensuring $(NAMESPACE) namespace exists..."; \
		kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -; \
	fi
	@echo "Checking if ck-intel-collector release exists in $(NAMESPACE) namespace..."
	@if helm list -n $(NAMESPACE) | grep -q ck-intel-collector; then \
		echo "Uninstalling existing ck-intel-collector release..."; \
		helm uninstall ck-intel-collector -n $(NAMESPACE); \
	else \
		echo "No existing release found, proceeding with fresh installation..."; \
	fi
	@sleep 4
	@echo "Installing ck-intel-collector using Helm in $(NAMESPACE) namespace..."
	@# Determine environment name and values file based on context and namespace
	@if [ "$(CONTEXT)" = "gcp-ck-qa" ] && [ "$(NAMESPACE)" = "codekarma" ]; then \
		ENV_NAME=gcp-ckqa; \
		VALUES_FILE=helm/values-gcp-ckqa.yaml; \
	elif [ "$(CONTEXT)" = "gcp-demo" ] && [ "$(NAMESPACE)" = "codekarma" ]; then \
		ENV_NAME=gcp-demo; \
		VALUES_FILE=helm/values-gcp-demo.yaml; \
	elif [ "$(CONTEXT)" = "cleartrip" ] && [ "$(NAMESPACE)" = "codekarma" ]; then \
		ENV_NAME=cleartrip; \
		VALUES_FILE=helm/values-gcp-cleartrip.yaml; \
	elif [ "$(CONTEXT)" = "gcp-flipkart" ] && [ "$(NAMESPACE)" = "codekarma" ]; then \
		ENV_NAME=flipkart; \
		VALUES_FILE=helm/values-gcp-flipkart.yaml; \
	fi; \
	echo "Using custom image: $(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)"; \
	echo "Using values file: $$VALUES_FILE"; \
	helm install ck-intel-collector open-telemetry/opentelemetry-collector \
		--version $(OTEL_HELM_CHART_VERSION) \
		-n $(NAMESPACE) \
		-f $$VALUES_FILE \
		--set image.repository="$(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME)" \
		--set image.tag="$(DOCKER_IMAGE_TAG)" \
		--set image.pullPolicy=Always \
		--set imagePullSecrets[0].name="ck-registry-secret" \
		--timeout 10m \
		--wait \
		--atomic
	@echo "GCP deployment completed successfully"

# Specific AWS deployment targets (call the generic build-and-deploy-aws-to-namespace with parameters)
build-and-deploy-aws-demo: ## Deploy to AWS demo cluster (aws-ck-qa context, demo namespace)
	$(MAKE) build-and-deploy-aws-to-namespace CONTEXT=aws-ck-qa NAMESPACE=demo

build-and-deploy-aws-dev: ## Deploy to AWS development cluster (aws-ck-qa context, codekarma namespace)
	$(MAKE) build-and-deploy-aws-to-namespace CONTEXT=aws-ck-qa NAMESPACE=ckqa

build-and-deploy-aws-prod: ## Deploy to AWS production cluster (aws-prod context, codekarma namespace)
	$(MAKE) build-and-deploy-aws-to-namespace CONTEXT=aws-prod NAMESPACE=codekarma

# Specific GCP deployment targets
build-and-deploy-gcp-ckqa: ## Deploy to GCP CKQA cluster (gcp-ck-qa context, codekarma namespace)
	$(MAKE) build-and-deploy-gcp-to-namespace CONTEXT=gcp-ck-qa NAMESPACE=codekarma

build-and-deploy-gcp-demo: ## Deploy to GCP demo cluster (gcp-demo context, demo namespace)
	$(MAKE) build-and-deploy-gcp-to-namespace CONTEXT=gcp-demo NAMESPACE=codekarma

build-and-deploy-gcp-flipkart: ## Deploy to GCP Flipkart cluster (gcp-flipkart context, codekarma namespace)
	$(MAKE) build-and-deploy-gcp-to-namespace CONTEXT=gcp-flipkart NAMESPACE=codekarma

deploy-gcp-flipkart: update-helm-repo ## Deploy to GCP Flipkart cluster without building (gcp-flipkart context, codekarma namespace)
	@echo "Deploying to flipkart cluster in codekarma namespace (without building)..."
	@$(MAKE) verify-context CONTEXT=gcp-flipkart
	@$(MAKE) validate-namespace CONTEXT=gcp-flipkart NAMESPACE=codekarma
	@$(MAKE) create-secrets CONTEXT=gcp-flipkart NAMESPACE=codekarma
	@if [ "codekarma" != "default" ]; then \
		echo "Ensuring codekarma namespace exists..."; \
		kubectl create namespace codekarma --dry-run=client -o yaml | kubectl apply -f -; \
	fi
	@echo "Checking if ck-intel-collector release exists in codekarma namespace..."
	@if helm list -n codekarma | grep -q ck-intel-collector; then \
		echo "Uninstalling existing ck-intel-collector release..."; \
		helm uninstall ck-intel-collector -n codekarma; \
	else \
		echo "No existing release found, proceeding with fresh installation..."; \
	fi
	@sleep 4
	@echo "Installing ck-intel-collector using Helm in codekarma namespace..."
	@ENV_NAME=flipkart; \
	VALUES_FILE=helm/values-gcp-flipkart.yaml; \
	echo "Using custom image: $(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)"; \
	echo "Using values file: $$VALUES_FILE"; \
	helm install ck-intel-collector open-telemetry/opentelemetry-collector \
		--version $(OTEL_HELM_CHART_VERSION) \
		-n codekarma \
		-f $$VALUES_FILE \
		--set image.repository="$(DOCKER_REGISTRY)/$(DOCKER_REGISTRY_USERNAME)/$$ENV_NAME/$(DOCKER_IMAGE_NAME)" \
		--set image.tag="$(DOCKER_IMAGE_TAG)" \
		--set image.pullPolicy=Always \
		--set imagePullSecrets[0].name="ck-registry-secret" \
		--timeout 10m \
		--wait \
		--atomic
	@echo "Flipkart deployment completed successfully"

# Build-only targets (build and push Docker image without deployment)
build-cleartrip: check-credentials ## Build and push Docker image for Cleartrip (cleartrip context, codekarma namespace)
	@echo "Building and pushing Docker image for Cleartrip..."
	@$(MAKE) verify-context CONTEXT=cleartrip
	@$(MAKE) dockerise CONTEXT=cleartrip NAMESPACE=codekarma
	@echo "Docker image built and pushed for Cleartrip successfully"

# Legacy targets (renamed for clarity)
ensure-kind-context: verify-context ## Alias for verify-context (deprecated, use verify-context)

update-helm-repo: ## Add/update OpenTelemetry Helm repository
	@echo "Adding/updating OpenTelemetry Helm repository..."
	@helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts 2>/dev/null || echo "Repository already exists, updating..."
	@helm repo update

load-kind: ## Load Docker image into kind cluster
	@echo "Loading Docker image into kind..."
	kind load docker-image $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)

# Generic kind deployment target (inlined from deploy-kind-to-namespace)
build-and-deploy-kind-to-namespace: verify-context validate-namespace dockerise update-helm-repo load-kind ## Deploy to kind cluster (use NAMESPACE parameter)
	@echo "Deploying to $(NAMESPACE) namespace in kind cluster..."
	@echo "Using values file: helm/values.yaml (local)"
	@echo "Uninstalling existing Helm chart (if present)..."
	@helm uninstall ck-intel-collector -n $(NAMESPACE) 2>/dev/null || echo "No existing release found, proceeding with fresh install..."
	sleep 3
	@echo "Installing Helm chart (version $(OTEL_HELM_CHART_VERSION))..."
	helm install ck-intel-collector open-telemetry/opentelemetry-collector --version $(OTEL_HELM_CHART_VERSION) -n $(NAMESPACE) -f helm/values.yaml --create-namespace
	@echo "Deployment to $(NAMESPACE) namespace complete."

build-and-deploy-kind-ckqa: ## Build Docker image and deploy to kind cluster in ckqa namespace
	$(MAKE) build-and-deploy-kind-to-namespace CONTEXT=kind NAMESPACE=ckqa

build-and-deploy-kind-demo: ## Build Docker image and deploy to kind cluster in demo namespace
	$(MAKE) build-and-deploy-kind-to-namespace CONTEXT=kind NAMESPACE=demo

build-and-deploy-kind-codekarma: ## Build Docker image and deploy to kind cluster in codekarma namespace
	$(MAKE) build-and-deploy-kind-to-namespace CONTEXT=kind NAMESPACE=codekarma

show-modules: ## Show all modules that will be tested/tidied
	@echo "Configured modules:"
	@for module_path in $(MODULES); do \
		module_name=$${module_path##*/}; \
		echo "  📁 $$module_path ($$module_name)"; \
	done

check-helm-versions: ## Check available OpenTelemetry Helm chart versions
	@echo "Current pinned version: $(OTEL_HELM_CHART_VERSION)"
	@echo ""
	@echo "Available versions:"
	@helm search repo open-telemetry/opentelemetry-collector --versions | head -10

components: build-ck-intel-collector ## Show available components
	@echo "Available components:"
	$(DIST_DIR)/$(BINARY_NAME) components

version: build-ck-intel-collector ## Show collector version
	$(DIST_DIR)/$(BINARY_NAME) --version

help: ## Display help
	@echo "ck-intel-collector Build and Deployment Targets"
	@echo "=============================================="
	@echo ""
	@echo "CONTEXT AND NAMESPACE MAPPING:"
	@echo "  kind context     → ckqa namespace (local development)"
	@echo "  aws-ck-qa context → codekarma namespace (AWS development environment)"
	@echo "  aws-ck-qa context → demo namespace (AWS demo environment)"
	@echo "  aws-prod context  → codekarma namespace (AWS production environment)"
	@echo "  gcp-ck-qa context → codekarma namespace (GCP CKQA environment)"
	@echo "  gcp-demo context  → codekarma namespace (GCP demo environment)"
	@echo "  gcp-flipkart context  → codekarma namespace (Flipkart environment)"
	@echo ""
	@echo "BUILD TARGETS:"
	@echo "  build-ck-intel-collector - Build the ck-intel-collector binary"
	@echo "  test                     - Run tests for all modules"
	@echo "  test-and-build          - Run tests and build the collector binary"
	@echo "  clean                   - Clean build artifacts"
	@echo "  tidy                    - Tidy up go modules for all modules"
	@echo ""
	@echo "DOCKER BUILD TARGETS:"
	@echo "  dockerise               - Build and push Docker image (context-aware, use CONTEXT and NAMESPACE)"
	@echo "  docker-run              - Run the collector in Docker"
	@echo ""
	@echo "LOCAL DEPLOYMENT (Kind):"
	@echo "  build-and-deploy-kind-ckqa - Build and deploy to kind cluster in ckqa namespace"
	@echo "  build-and-deploy-kind-demo - Build and deploy to kind cluster in demo namespace"
	@echo "  build-and-deploy-kind-codekarma - Build and deploy to kind cluster in codekarma namespace"
	@echo ""
	@echo "AWS DEPLOYMENT (Specific Targets):"
	@echo "  build-and-deploy-aws-demo - Deploy to AWS demo cluster (aws-ck-qa context, demo namespace)"
	@echo "  build-and-deploy-aws-dev - Deploy to AWS development cluster (aws-ck-qa context, codekarma namespace)"
	@echo "  build-and-deploy-aws-prod - Deploy to AWS production cluster (aws-prod context, codekarma namespace)"
	@echo ""
	@echo "GCP DEPLOYMENT (Specific Targets):"
	@echo "  build-and-deploy-gcp-ckqa - Deploy to GCP CKQA cluster (gcp-ck-qa context, codekarma namespace)"
	@echo "  build-and-deploy-gcp-demo - Deploy to GCP demo cluster (gcp-demo context, codekarma namespace)"
	@echo "  build-and-deploy-gcp-flipkart - Deploy to GCP Flipkart cluster (gcp-flipkart context, codekarma namespace)"
	@echo "  deploy-gcp-flipkart - Deploy to GCP Flipkart cluster without building (gcp-flipkart context, codekarma namespace)"
	@echo ""
	@echo "BUILD-ONLY TARGETS (Build and push Docker image without deployment):"
	@echo "  build-cleartrip - Build and push Docker image for Cleartrip (cleartrip context, codekarma namespace)"
	@echo ""
	@echo "GENERIC TARGETS (Require Parameters):"
	@echo "  verify-context          - Verify kubectl context (use CONTEXT parameter)"
	@echo "  create-secrets          - Create/update registry secrets (use CONTEXT and NAMESPACE parameters)"
	@echo "  dockerise               - Build and push Docker image (use CONTEXT and NAMESPACE parameters)"
	@echo "  build-and-deploy-aws-to-namespace - Generic AWS deployment (use CONTEXT and NAMESPACE parameters)"
	@echo "  build-and-deploy-gcp-to-namespace - Generic GCP deployment (use CONTEXT and NAMESPACE parameters)"
	@echo "  build-and-deploy-kind-to-namespace - Generic kind deployment (use NAMESPACE parameter)"
	@echo ""
	@echo "CONTEXT VERIFICATION:"
	@echo "  verify-context CONTEXT=kind      - Verify kind context"
	@echo "  verify-context CONTEXT=aws-ck-qa - Verify AWS CKQA context (aws*ck-qa pattern)"
	@echo "  verify-context CONTEXT=aws-prod  - Verify AWS production context (aws-prod pattern)"
	@echo "  verify-context CONTEXT=gcp-ck-qa - Verify GCP CKQA context (GKE or gcp pattern)"
	@echo "  verify-context CONTEXT=gcp-demo  - Verify GCP demo context (gke_resounding-node-471205-f9_us-east1_demo-cluster pattern)"
	@echo "  verify-context CONTEXT=cleartrip - Verify Cleartrip context (no validation)"
	@echo "  verify-context CONTEXT=gcp-flipkart  - Verify Flipkart context (gke_fk-code-karma_asia-south1_gke-code-karma-prod-1)"
	@echo ""
	@echo "SECRET CREATION:"
	@echo "  create-secrets CONTEXT=aws-ck-qa NAMESPACE=demo      - Create secrets for demo"
	@echo "  create-secrets CONTEXT=aws-ck-qa NAMESPACE=codekarma  - Create secrets for development"
	@echo "  create-secrets CONTEXT=aws-prod NAMESPACE=codekarma   - Create secrets for production"
	@echo "  create-secrets CONTEXT=gcp-ck-qa NAMESPACE=codekarma  - Create secrets for GCP CKQA"
	@echo "  create-secrets CONTEXT=gcp-demo NAMESPACE=codekarma        - Create secrets for GCP demo"
	@echo "  create-secrets CONTEXT=cleartrip NAMESPACE=codekarma       - Create secrets for Cleartrip"
	@echo "  create-secrets CONTEXT=gcp-flipkart NAMESPACE=codekarma         - Create secrets for Flipkart"
	@echo ""
	@echo "DOCKER BUILD:"
	@echo "  dockerise CONTEXT=kind NAMESPACE=ckqa               - Build for kind context (local)"
	@echo "  dockerise CONTEXT=aws-ck-qa NAMESPACE=demo          - Build for demo environment"
	@echo "  dockerise CONTEXT=aws-ck-qa NAMESPACE=codekarma     - Build for development environment"
	@echo "  dockerise CONTEXT=aws-prod NAMESPACE=codekarma      - Build for production environment"
	@echo "  dockerise CONTEXT=gcp-ck-qa NAMESPACE=codekarma     - Build for GCP CKQA environment"
	@echo "  dockerise CONTEXT=gcp-demo NAMESPACE=codekarma           - Build for GCP demo environment"
	@echo "  dockerise CONTEXT=cleartrip NAMESPACE=codekarma          - Build for Cleartrip environment"
	@echo "  dockerise CONTEXT=gcp-flipkart NAMESPACE=codekarma           - Build for Flipkart environment"
	@echo ""
	@echo "UTILITIES:"
	@echo "  show-modules            - Show all modules that will be tested/tidied"
	@echo "  check-helm-versions     - Check available OpenTelemetry Helm chart versions"
	@echo "  components              - Show available components"
	@echo "  version                 - Show collector version"
	@echo "  update-helm-repo        - Add/update OpenTelemetry Helm repository"
	@echo "  help                    - Display this help"
	@echo ""
	@echo "VALUES FILES BY ENVIRONMENT:"
	@echo "  Local (Kind):     helm/values.yaml"
	@echo "  AWS Demo:         helm/values-aws-demo.yaml"
	@echo "  AWS Development:  helm/values-aws-ckqa.yaml"
	@echo "  AWS Production:   helm/values-aws-prod.yaml"
	@echo "  GCP CKQA:         helm/values-gcp-ckqa.yaml"
	@echo "  GCP Demo:         helm/values-gcp-demo.yaml"
	@echo "  GCP Cleartrip:    helm/values-gcp-cleartrip.yaml"
	@echo "  GCP Flipkart:     helm/values-gcp-flipkart.yaml"
	@echo ""
	@echo "ENVIRONMENT VARIABLES (optional - defaults provided):"
	@echo "  DOCKER_REGISTRY_USERNAME - Your registry username (default: sabareesh-ckt)"
	@echo "  DOCKER_REGISTRY_TOKEN    - Your registry token/password (default: built-in token)"
	@echo ""
	@echo "EXAMPLES:"
	@echo "  # Specific deployment targets (recommended)"
	@echo "  make build-and-deploy-aws-demo                    # Demo environment"
	@echo "  make build-and-deploy-aws-dev                     # Development environment"
	@echo "  make build-and-deploy-aws-prod                    # Production environment"
	@echo "  make build-and-deploy-gcp-ckqa                    # GCP CKQA environment"
	@echo "  make build-and-deploy-gcp-demo                    # GCP demo environment"
	@echo "  make build-and-deploy-gcp-flipkart                # GCP Flipkart environment"
	@echo "  make deploy-gcp-flipkart                          # Deploy to GCP Flipkart without building"
	@echo "  make build-and-deploy-kind-ckqa                   # Local development"
	@echo "  make build-cleartrip                              # Build and push for Cleartrip"
	@echo ""
	@echo "  # Generic targets with parameters"
	@echo "  make verify-context CONTEXT=aws-ck-qa"
	@echo "  make verify-context CONTEXT=gcp-ck-qa"
	@echo "  make verify-context CONTEXT=gcp-demo"
	@echo "  make verify-context CONTEXT=cleartrip"
	@echo "  make verify-context CONTEXT=gcp-flipkart"
	@echo "  make create-secrets CONTEXT=aws-ck-qa NAMESPACE=demo"
	@echo "  make create-secrets CONTEXT=gcp-ck-qa NAMESPACE=codekarma"
	@echo "  make create-secrets CONTEXT=gcp-demo NAMESPACE=codekarma"
	@echo "  make create-secrets CONTEXT=cleartrip NAMESPACE=codekarma"
	@echo "  make create-secrets CONTEXT=gcp-flipkart NAMESPACE=codekarma"
	@echo "  make dockerise CONTEXT=aws-ck-qa NAMESPACE=demo"
	@echo "  make dockerise CONTEXT=gcp-ck-qa NAMESPACE=codekarma"
	@echo "  make dockerise CONTEXT=gcp-demo NAMESPACE=codekarma"
	@echo "  make dockerise CONTEXT=cleartrip NAMESPACE=codekarma"
	@echo "  make dockerise CONTEXT=gcp-flipkart NAMESPACE=codekarma"
	@echo "  make build-and-deploy-aws-to-namespace CONTEXT=aws-ck-qa NAMESPACE=demo"
	@echo "  make build-and-deploy-gcp-to-namespace CONTEXT=gcp-ck-qa NAMESPACE=codekarma"
	@echo "  make build-and-deploy-gcp-to-namespace CONTEXT=gcp-demo NAMESPACE=codekarma"
	@echo "  make build-and-deploy-gcp-to-namespace CONTEXT=gcp-flipkart NAMESPACE=codekarma"
	@echo ""
	@echo "  # Override credentials"
	@echo "  export DOCKER_REGISTRY_USERNAME=your-username"
	@echo "  export DOCKER_REGISTRY_TOKEN=your-token"
	@echo "  make build-and-deploy-aws-demo"
	@echo "  make build-and-deploy-gcp-ckqa"
	@echo "  make build-and-deploy-gcp-demo"
	@echo "  make build-and-deploy-gcp-flipkart"
	@echo "  make deploy-gcp-flipkart"
	@echo "  make build-cleartrip" 
