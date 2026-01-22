# GitHub Actions Workflows Documentation

This document provides comprehensive documentation for all GitHub Actions workflows in the `ck-otel-collector` repository. These workflows are designed to handle building, testing, and deploying the OpenTelemetry Collector to different environments.

## 📋 Workflow Overview

| Workflow | Purpose | Environment | Trigger | Key Features |
|----------|---------|-------------|---------|--------------|
| **build-and-deploy-prod.yml** | Build production images with versioning | Production | Manual | Git tagging, version management, optional auto-deploy |
| **build-and-deploy-demo.yml** | Build demo images for testing | Demo | Manual | Commit-based tagging, demo environment |
| **deploy-prod.yml** | Deploy existing images to production | Production | Manual | Production safety checks, image validation |
| **deploy-demo.yml** | Deploy existing images to demo | Demo | Manual | Demo deployment, simpler validation |
| **rollout-restart.yml** | Restart deployments without redeploying | Both | Manual | Zero-downtime restarts |

## 🚀 Workflow Details

### 1. Build Production (Optional Deploy) - `build-and-deploy-prod.yml`

**Purpose**: Build production-ready Docker images with semantic versioning and optional automatic deployment.

**When to Use**:
- Creating new production releases
- Building from the `githubActions` branch
- When you want to increment version numbers automatically
- When you need to run tests before building

**Key Features**:
- ✅ **Branch Protection**: Only runs on `main` branch
- 🏷️ **Automatic Versioning**: Increments Git tags (v1.0.0, v1.0.1, etc.)
- 🧪 **Optional Testing**: Can run tests before building
- 🚀 **Optional Auto-deploy**: Can trigger deployment after successful build
- 🧹 **Dependency Management**: Automatically tidies Go modules
- 🔄 **Rollback Safety**: Deletes Git tags if build fails

**Inputs**:
- `run_tests`: Enable/disable testing (default: true)
- `auto_deploy`: Trigger deployment after build (default: false)
- `deploy_provider`: Cloud provider for deployment (default: aws)
- `production_confirmation`: Type "deploy in prod" to confirm

**Workflow Steps**:
1. **Validate Inputs**: Check branch and input validation
2. **Run Tests**: Execute tests for all modules (if enabled)
3. **Create Git Tag**: Calculate and create new version tag
4. **Build & Push**: Build multi-arch Docker image and push to registry
5. **Cleanup**: Remove Git tag if build fails
6. **Summary**: Create build summary with next steps
7. **Auto-deploy**: Trigger deployment workflow if enabled

**Outputs**:
- Docker image with semantic version tag
- Git tag for version tracking
- Build summary with deployment instructions

---

### 2. Build Demo (Optional Deploy) - `build-and-deploy-demo.yml`

**Purpose**: Build demo/testing Docker images with commit-based tagging for rapid iteration.

**When to Use**:
- Creating demo builds for testing
- Rapid iteration without version management
- When you need to test changes quickly
- Building from the `githubActions` branch

**Key Features**:
- ✅ **Branch Protection**: Only runs on `demo` branch
- 🏷️ **Commit-based Tagging**: Uses branch-commit format (e.g., `demo-a1b2c3d4`)
- 🧪 **Optional Testing**: Can run tests before building
- 🚀 **Optional Auto-deploy**: Can trigger demo deployment after build
- 🔄 **No Git Tags**: Doesn't create permanent Git tags
- ⚡ **Fast Iteration**: Ideal for development and testing

**Inputs**:
- `run_tests`: Enable/disable testing (default: true)
- `auto_deploy`: Trigger deployment after build (default: false)
- `deploy_provider`: Cloud provider for deployment (default: aws)
- `demo_confirmation`: Type "deploy in demo" to confirm

**Workflow Steps**:
1. **Validate Inputs**: Check branch and input validation
2. **Run Tests**: Execute tests for all modules (if enabled)
3. **Generate Tag**: Create commit-based image tag
4. **Build & Push**: Build multi-arch Docker image and push to registry
5. **Summary**: Create build summary with next steps
6. **Auto-deploy**: Trigger demo deployment workflow if enabled

**Outputs**:
- Docker image with commit-based tag
- Build summary with deployment instructions
- No permanent Git tags created

---

### 3. Deploy Production - `deploy-prod.yml`

**Purpose**: Deploy existing Docker images to production environments with strict safety checks.

**When to Use**:
- Deploying pre-built images to production
- When you have a specific image tag to deploy
- Production deployments requiring safety confirmation
- When you need to validate image existence before deployment

**Key Features**:
- 🔒 **Production Safety**: Requires typing "deploy in prod" to confirm
- 🏷️ **Image Validation**: Verifies image tag exists before deployment
- 🎯 **Version Fetching**: Shows available versions and helps select correct tag
- 🚀 **Rollout Restart**: Optional restart after deployment for same-image config changes
- 📊 **Deployment Summary**: Comprehensive deployment report
- 🔄 **Error Handling**: Stops pipeline on deployment failures

**Inputs**:
- `deploy_provider`: Cloud provider (default: aws)
- `image_tag`: Image version to deploy (required)
- `production_confirmation`: Type "deploy in prod" (required)
- `trigger_restart_after_deploy`: Enable rollout restart (default: false)

**Workflow Steps**:
1. **Fetch Versions**: Get available image versions and validate selection
2. **Validate Confirmation**: Ensure production deployment is confirmed
3. **Validate Image**: Check if selected image tag exists
4. **Parse Target**: Determine deployment target and namespace
5. **Deploy**: Deploy to Kubernetes using Helm
6. **Summary**: Create deployment summary
7. **Rollout Restart**: Trigger restart if enabled

**Outputs**:
- Successful deployment to production cluster
- Deployment summary with all details
- Optional rollout restart for configuration changes

---

### 4. Deploy Demo - `deploy-demo.yml`

**Purpose**: Deploy existing Docker images to demo environments with simplified validation.

**When to Use**:
- Deploying pre-built images to demo environment
- Testing deployments before production
- Demo environment updates
- When you need simpler deployment validation

**Key Features**:
- 🔒 **Demo Confirmation**: Requires typing "deploy in demo" to confirm
- 🎯 **Simplified Validation**: Less strict than production deployment
- 🚀 **Rollout Restart**: Optional restart after deployment
- 📊 **Deployment Summary**: Comprehensive demo deployment report
- 🔄 **Error Handling**: Stops pipeline on deployment failures

**Inputs**:
- `deploy_provider`: Cloud provider (default: aws)
- `image_tag`: Image tag to deploy (required)
- `demo_confirmation`: Type "deploy in demo" (required)
- `trigger_restart_after_deploy`: Enable rollout restart (default: false)

**Workflow Steps**:
1. **Validate Confirmation**: Ensure demo deployment is confirmed
2. **Parse Target**: Determine deployment target and namespace
3. **Deploy**: Deploy to Kubernetes using Helm
4. **Summary**: Create deployment summary
5. **Rollout Restart**: Trigger restart if enabled

**Outputs**:
- Successful deployment to demo cluster
- Deployment summary with all details
- Optional rollout restart for configuration changes

---

### 5. Rollout Restart - `rollout-restart.yml`

**Purpose**: Restart Kubernetes deployments without redeploying the entire application.

**When to Use**:
- Restarting pods without changing images
- Applying configuration changes that require pod restart
- Zero-downtime application restarts
- When you need to refresh pod state

**Key Features**:
- 🔄 **Zero Downtime**: Restarts pods without full redeployment
- 🌍 **Environment Support**: Works with both prod and demo
- 🚀 **Provider Agnostic**: Supports AWS and other providers
- 📊 **Reusable Workflow**: Uses shared CI/CD commons workflow

**Inputs**:
- `environment`: Select environment (prod/demo)
- `provider`: Cloud provider (default: aws)

**Workflow Steps**:
1. **Determine Environment**: Set environment-specific variables
2. **Call Reusable Workflow**: Execute shared rollout restart workflow

**Outputs**:
- Successful pod restart
- No image changes or redeployment

---

## 🏗️ Infrastructure Requirements

### Prerequisites

1. **GitHub Secrets**:
   - `DOCKER_REGISTRY_USERNAME`: GitHub username for container registry
   - `DOCKER_REGISTRY_TOKEN`: Personal access token for container registry
   - `AWS_ACCESS_KEY_ID`: AWS access key for cluster access
   - `AWS_SECRET_ACCESS_KEY`: AWS secret key for cluster access

2. **Kubernetes Cluster**:
   **Environment Details**:
- **Production**: 
  - EKS cluster named `ck-prod` in `ap-south-1` region
  - Namespace: `codekarma` for production
  - Branch: `main`
- **Demo**: 
  - EKS cluster named `ck-qa` in `us-east-2` region
  - Namespace: `demo` for demo environment
  - Branch: `demo`

3. **Container Registry**:
   - GitHub Container Registry (ghcr.io)
   - Repository structure: `ghcr.io/{username}/{env}/{image-name}`

### Helm Values Files

The workflows expect the following Helm values files:
- `helm/values-aws-prod.yaml` - Production AWS configuration
- `helm/values-aws-demo.yaml` - Demo AWS configuration

---

## 🚀 Onboarding New Providers/Clusters

### Adding a New Cloud Provider

1. **Update Workflow Files**:
   ```yaml
   # In workflow inputs
   deploy_provider:
     description: 'Select cloud provider for deployment'
     required: true
     default: 'aws'
     type: choice
     options:
     - aws
     - gcp  # Add new provider
     - azure # Add new provider
   ```

2. **Add Provider-Specific Logic**:
   ```yaml
   # In parse-target step
   if [[ "$PROVIDER" = "gcp" ]]; then
     # GCP-specific configuration
     NAMESPACE="${{ env.GCP_NAMESPACE }}"
     CLUSTER_NAME="${{ env.GCP_CLUSTER_NAME }}"
   elif [[ "$PROVIDER" = "azure" ]]; then
     # Azure-specific configuration
     NAMESPACE="${{ env.AZURE_NAMESPACE }}"
     CLUSTER_NAME="${{ env.AZURE_CLUSTER_NAME }}"
   fi
   ```

3. **Add Provider Credentials**:
   ```yaml
   # For GCP
   - name: Configure GCP credentials
     if: steps.parse-target.outputs.provider == 'gcp'
     uses: google-github-actions/auth@v1
     with:
       credentials_json: ${{ secrets.GCP_SA_KEY }}
   ```

4. **Update Environment Variables**:
   ```yaml
   env:
     # GCP Configuration
     GCP_PROJECT_ID: ${{ secrets.GCP_PROJECT_ID }}
     GCP_CLUSTER_NAME: ${{ secrets.GCP_CLUSTER_NAME }}
     GCP_REGION: ${{ secrets.GCP_REGION }}
   ```

### Adding a New Cluster

1. **Create Helm Values File**:
   ```bash
   # Create new values file
   cp helm/values-aws-prod.yaml helm/values-aws-newcluster.yaml
   ```

2. **Update Workflow Environment Variables**:
   ```yaml
   env:
     # New cluster configuration
     NEW_CLUSTER_NAMESPACE: newcluster
     NEW_CLUSTER_NAME: ck-newcluster
     NEW_CLUSTER_REGION: us-west-2
   ```

3. **Update Cluster Selection Logic**:
   ```yaml
   # In parse-target step
   if [[ "$CLUSTER" = "newcluster" ]]; then
     NAMESPACE="${{ env.NEW_CLUSTER_NAMESPACE }}"
     CLUSTER_NAME="${{ env.NEW_CLUSTER_NAME }}"
     REGION="${{ env.NEW_CLUSTER_REGION }}"
   fi
   ```

4. **Add Cluster-Specific Secrets**:
   ```yaml
   # Add new secrets to GitHub repository
   NEW_CLUSTER_AWS_ACCESS_KEY_ID
   NEW_CLUSTER_AWS_SECRET_ACCESS_KEY
   ```

---

## 🔧 Configuration Management

### Environment Variables

**Environment Variables**:
- **Production Environment**:
  - `CLUSTER_ENV`: prod
  - `PROD_NAMESPACE`: codekarma
  - `EKS_CLUSTER_NAME`: ck-prod
  - `AWS_REGION`: ap-south-1
  - `MAIN_BRANCH`: main

- **Demo Environment**:
  - `CLUSTER_ENV`: demo
  - `DEMO_NAMESPACE`: demo
  - `EKS_CLUSTER_NAME`: ck-qa
  - `AWS_REGION`: us-east-2
  - `DEMO_BRANCH`: demo

### Helm Chart Configuration

- **Chart**: `open-telemetry/opentelemetry-collector`
- **Version**: `0.126.0`
- **Release Name**: `ck-intel-collector`
- **Image Pull Secrets**: `ck-registry-secret`

### Docker Image Configuration

- **Registry**: `ghcr.io`
- **Image Name**: `ck-intel-collector`
- **Multi-arch**: `linux/amd64`, `linux/arm64`
- **Builder**: OpenTelemetry Collector Builder v0.128.0

---

## 🚨 Safety Features

### Production Deployments

1. **Confirmation Required**: Must type exact confirmation text
2. **Branch Protection**: Only runs on specified branches
3. **Image Validation**: Verifies image exists before deployment
4. **Environment Protection**: Uses GitHub environments for approval
5. **Rollback Capability**: Can rollback to previous versions

### Demo Deployments

1. **Confirmation Required**: Must type exact confirmation text
2. **Branch Protection**: Only runs on specified branches
3. **Simplified Validation**: Less strict than production
4. **No Version Management**: Uses commit-based tagging

---

## 📊 Monitoring and Troubleshooting

### Workflow Monitoring

1. **GitHub Actions**: Monitor workflow runs in Actions tab
2. **Deployment Status**: Check deployment summaries
3. **Image Registry**: Verify images in container registry
4. **Kubernetes**: Monitor pod status and logs

### Common Issues

1. **Authentication Failures**:
   - Check GitHub secrets are properly configured
   - Verify AWS credentials have correct permissions

2. **Build Failures**:
   - Review Go module dependencies
   - Check Docker build context and Dockerfile

3. **Deployment Failures**:
   - Verify Helm values file exists
   - Check Kubernetes cluster connectivity
   - Review namespace and resource quotas

4. **Image Pull Failures**:
   - Verify image tag exists in registry
   - Check registry authentication secrets

---

## 🔄 Workflow Dependencies

### Build → Deploy Flow

```
build-and-deploy-prod.yml → deploy-prod.yml
build-and-deploy-demo.yml → deploy-demo.yml
```

### Deploy → Restart Flow

```
deploy-prod.yml → rollout-restart.yml
deploy-demo.yml → rollout-restart.yml
```

### Manual Trigger Flow

```
deploy-prod.yml → rollout-restart.yml (optional)
deploy-demo.yml → rollout-restart.yml (optional)
```

---

## 📝 Best Practices

1. **Always Test**: Run tests before building production images
2. **Use Semantic Versioning**: Follow versioning conventions for production
3. **Confirm Deployments**: Double-check deployment targets and confirmations
4. **Monitor Deployments**: Watch deployment progress and verify success
5. **Use Rollout Restart**: For configuration changes without image updates
6. **Keep Secrets Secure**: Rotate credentials regularly and use least privilege
7. **Document Changes**: Update this documentation when adding new providers/clusters

---

## 🔗 Related Resources

- [OpenTelemetry Collector Documentation](https://opentelemetry.io/docs/collector/)
- [Helm Charts Repository](https://github.com/open-telemetry/opentelemetry-helm-charts)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Kubernetes Deployment Strategies](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
- [AWS EKS Documentation](https://docs.aws.amazon.com/eks/)

---

## 📞 Support

For questions or issues with these workflows:

1. Check the workflow logs in GitHub Actions
2. Review the deployment summaries for error details
3. Verify configuration and secrets are correct
4. Contact the DevOps team for assistance

---

*Last updated: $(date)*
*Workflow version: v1.0*
