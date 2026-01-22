#!/bin/bash

# GitHub Actions Setup Script for OTEL Collector
# This script helps configure the required secrets and environment variables

set -e

echo "🚀 GitHub Actions Setup for OTEL Collector"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Check if required tools are installed
check_requirements() {
    print_info "Checking requirements..."
    
    if ! command -v aws &> /dev/null; then
        print_error "AWS CLI is not installed. Please install it first."
        exit 1
    fi
    
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed. Please install it first."
        exit 1
    fi
    
    if ! command -v gh &> /dev/null; then
        print_warning "GitHub CLI is not installed. You'll need to configure secrets manually."
    fi
    
    print_status "Requirements check completed"
}

# Get AWS configuration for environment-based setup
get_aws_config() {
    print_info "Getting AWS configuration for environment-based setup..."
    
    # Get AWS account ID
    AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text 2>/dev/null || echo "")
    if [ -z "$AWS_ACCOUNT_ID" ]; then
        print_error "Failed to get AWS account ID. Please run 'aws configure' first."
        exit 1
    fi
    
    # Get AWS region
    AWS_REGION=$(aws configure get region 2>/dev/null || echo "")
    if [ -z "$AWS_REGION" ]; then
        print_error "AWS region not configured. Please run 'aws configure' first."
        exit 1
    fi
    
    # Get EKS clusters
    print_info "Available EKS clusters in region $AWS_REGION:"
    aws eks list-clusters --region $AWS_REGION --query 'clusters[]' --output table
    
    echo ""
    print_info "Configuring environment-based AWS setup..."
    
    # Get default AWS credentials
    AWS_ACCESS_KEY_ID=$(aws configure get aws_access_key_id)
    AWS_SECRET_ACCESS_KEY=$(aws configure get aws_secret_access_key)
    
    # Get EKS cluster name
    read -p "Enter EKS cluster name: " EKS_CLUSTER_NAME
    
    print_status "AWS configuration retrieved for environment-based setup"
}

# Get GCP configuration for environment-based setup
get_gcp_config() {
    print_info "Getting GCP configuration for environment-based setup..."
    
    # Check if gcloud is installed
    if ! command -v gcloud &> /dev/null; then
        print_warning "gcloud CLI is not installed. GCP configuration will be skipped."
        return
    fi
    
    # Get current project
    GCP_PROJECT_ID=$(gcloud config get-value project 2>/dev/null || echo "")
    if [ -z "$GCP_PROJECT_ID" ]; then
        print_error "GCP project not configured. Please run 'gcloud config set project <project-id>' first."
        return
    fi
    
    # Get GKE clusters
    print_info "Available GKE clusters in project $GCP_PROJECT_ID:"
    gcloud container clusters list --format="table(name,location,status)" 2>/dev/null || echo "No clusters found or access denied"
    
    echo ""
    print_info "Configuring environment-based GCP setup..."
    
    # Get GKE cluster name
    read -p "Enter GKE cluster name: " GKE_CLUSTER_NAME
    read -p "Enter GCP region: " GCP_REGION
    
    # Generate service account key
    print_info "Generating GCP service account key..."
    gcloud iam service-accounts create github-actions --display-name="GitHub Actions" 2>/dev/null || echo "Service account already exists"
    gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
        --member="serviceAccount:github-actions@$GCP_PROJECT_ID.iam.gserviceaccount.com" \
        --role="roles/container.admin" 2>/dev/null || echo "Policy binding failed"
    
    gcloud iam service-accounts keys create /tmp/gcp-key.json \
        --iam-account=github-actions@$GCP_PROJECT_ID.iam.gserviceaccount.com
    GCP_SA_KEY=$(cat /tmp/gcp-key.json)
    rm /tmp/gcp-key.json
    
    print_status "GCP configuration retrieved for environment-based setup"
}

# Get GitHub repository information
get_github_config() {
    print_info "Getting GitHub repository information..."
    
    # Get repository name
    if [ -d ".git" ]; then
        REPO_URL=$(git remote get-url origin 2>/dev/null || echo "")
        if [[ $REPO_URL == *"github.com"* ]]; then
            REPO_NAME=$(echo $REPO_URL | sed 's/.*github\.com[:/]\([^/]*\/[^/]*\)\.git.*/\1/')
        else
            REPO_NAME=""
        fi
    else
        REPO_NAME=""
    fi
    
    if [ -z "$REPO_NAME" ]; then
        read -p "Enter GitHub repository (format: owner/repo): " REPO_NAME
    fi
    
    print_status "GitHub repository: $REPO_NAME"
}

# Generate AWS credentials
generate_aws_credentials() {
    print_info "Generating AWS credentials..."
    
    # Check if AWS credentials are configured
    if ! aws sts get-caller-identity &> /dev/null; then
        print_error "AWS credentials not configured. Please run 'aws configure' first."
        exit 1
    fi
    
    # Get access key and secret
    AWS_ACCESS_KEY_ID=$(aws configure get aws_access_key_id)
    AWS_SECRET_ACCESS_KEY=$(aws configure get aws_secret_access_key)
    
    print_status "AWS credentials retrieved"
}

# Create GitHub secrets for environment-based setup
create_github_secrets() {
    print_info "Creating GitHub secrets for environment-based setup..."
    
    if command -v gh &> /dev/null; then
        # Use GitHub CLI to set secrets
        print_info "Using GitHub CLI to set secrets..."
        
        # AWS secrets (repository level)
        print_info "Setting AWS secrets (repository level)..."
        gh secret set AWS_ACCESS_KEY_ID --body "$AWS_ACCESS_KEY_ID" --repo "$REPO_NAME"
        gh secret set AWS_SECRET_ACCESS_KEY --body "$AWS_SECRET_ACCESS_KEY" --repo "$REPO_NAME"
        gh secret set AWS_REGION --body "$AWS_REGION" --repo "$REPO_NAME"
        gh secret set EKS_CLUSTER_NAME --body "$EKS_CLUSTER_NAME" --repo "$REPO_NAME"
        
        # GCP secrets (if available)
        if [ -n "$GCP_PROJECT_ID" ] && [ -n "$GKE_CLUSTER_NAME" ] && [ -n "$GCP_REGION" ]; then
            print_info "Setting GCP secrets (repository level)..."
            gh secret set GCP_PROJECT_ID --body "$GCP_PROJECT_ID" --repo "$REPO_NAME"
            gh secret set GKE_CLUSTER_NAME --body "$GKE_CLUSTER_NAME" --repo "$REPO_NAME"
            gh secret set GCP_REGION --body "$GCP_REGION" --repo "$REPO_NAME"
            gh secret set GCP_SA_KEY --body "$GCP_SA_KEY" --repo "$REPO_NAME"
        fi
        
        print_status "GitHub secrets created successfully for environment-based setup"
    else
        # Manual instructions
        print_warning "GitHub CLI not available. Please set secrets manually:"
        echo ""
        echo "Go to: https://github.com/$REPO_NAME/settings/secrets/actions"
        echo ""
        echo "Add the following AWS secrets (repository level):"
        echo "  AWS_ACCESS_KEY_ID: $AWS_ACCESS_KEY_ID"
        echo "  AWS_SECRET_ACCESS_KEY: $AWS_SECRET_ACCESS_KEY"
        echo "  AWS_REGION: $AWS_REGION"
        echo "  EKS_CLUSTER_NAME: $EKS_CLUSTER_NAME"
        echo ""
        if [ -n "$GCP_PROJECT_ID" ] && [ -n "$GKE_CLUSTER_NAME" ] && [ -n "$GCP_REGION" ]; then
            echo "Add the following GCP secrets (repository level):"
            echo "  GCP_PROJECT_ID: $GCP_PROJECT_ID"
            echo "  GKE_CLUSTER_NAME: $GKE_CLUSTER_NAME"
            echo "  GCP_REGION: $GCP_REGION"
            echo "  GCP_SA_KEY: [Service account key JSON]"
            echo ""
        fi
    fi
}

# Create GitHub environments
create_github_environments() {
    print_info "Creating GitHub environments..."
    
    if command -v gh &> /dev/null; then
        # Create environments using GitHub CLI
        print_info "Creating environments: demo, dev, prod"
        
        # Note: GitHub CLI doesn't have direct environment creation
        # Environments need to be created manually in the GitHub UI
        print_warning "Please create environments manually in GitHub:"
        echo "  Go to: https://github.com/$REPO_NAME/settings/environments"
        echo "  Create environments: demo, dev, prod"
    else
        print_warning "Please create environments manually in GitHub:"
        echo "  Go to: https://github.com/$REPO_NAME/settings/environments"
        echo "  Create environments: demo, dev, prod"
    fi
}

# Test AWS connectivity for environment-based setup
test_aws_connectivity() {
    print_info "Testing AWS connectivity for environment-based setup..."
    
    # Test EKS cluster access
    print_info "Testing EKS cluster access..."
    if aws eks describe-cluster --name "$EKS_CLUSTER_NAME" --region "$AWS_REGION" &> /dev/null; then
        print_status "EKS cluster access verified"
    else
        print_error "Cannot access EKS cluster. Please check your AWS credentials and cluster name."
        exit 1
    fi
    
    # Test kubectl access
    print_info "Testing kubectl access..."
    aws eks update-kubeconfig --name "$EKS_CLUSTER_NAME" --region "$AWS_REGION"
    if kubectl cluster-info &> /dev/null; then
        print_status "Kubernetes cluster access verified"
    else
        print_error "Cannot access Kubernetes cluster. Please check your kubectl configuration."
        exit 1
    fi
}

# Generate configuration summary for environment-based setup
generate_summary() {
    print_info "Generating configuration summary for environment-based setup..."
    
    cat > github-actions-config.md << EOF
# GitHub Actions Configuration Summary (Environment-Based)

## Repository Information
- **Repository**: $REPO_NAME
- **AWS Account**: $AWS_ACCOUNT_ID

## Repository-Level Secrets
The following secrets are configured at the repository level:

### AWS Secrets
- \`AWS_ACCESS_KEY_ID\`: $AWS_ACCESS_KEY_ID
- \`AWS_SECRET_ACCESS_KEY\`: [REDACTED]
- \`AWS_REGION\`: $AWS_REGION
- \`EKS_CLUSTER_NAME\`: $EKS_CLUSTER_NAME
EOF

    # Add GCP configuration if available
    if [ -n "$GCP_PROJECT_ID" ] && [ -n "$GKE_CLUSTER_NAME" ] && [ -n "$GCP_REGION" ]; then
        cat >> github-actions-config.md << EOF

### GCP Secrets
- \`GCP_PROJECT_ID\`: $GCP_PROJECT_ID
- \`GKE_CLUSTER_NAME\`: $GKE_CLUSTER_NAME
- \`GCP_REGION\`: $GCP_REGION
- \`GCP_SA_KEY\`: [Service account key JSON]
EOF
    fi

    cat >> github-actions-config.md << EOF

## Environment-Based Secrets
You need to create the following environments in GitHub and set environment-specific secrets:

### Demo Environment
**Create environment**: \`demo\`
**Environment secrets** (same names, different values):
- \`AWS_ACCESS_KEY_ID\`: [Demo AWS Access Key]
- \`AWS_SECRET_ACCESS_KEY\`: [Demo AWS Secret Key]
- \`AWS_REGION\`: [Demo AWS Region]
- \`EKS_CLUSTER_NAME\`: [Demo EKS Cluster Name]

### Development Environment
**Create environment**: \`dev\`
**Environment secrets** (same names, different values):
- \`AWS_ACCESS_KEY_ID\`: [Dev AWS Access Key]
- \`AWS_SECRET_ACCESS_KEY\`: [Dev AWS Secret Key]
- \`AWS_REGION\`: [Dev AWS Region]
- \`EKS_CLUSTER_NAME\`: [Dev EKS Cluster Name]

### Production Environment
**Create environment**: \`prod\`
**Environment secrets** (same names, different values):
- \`AWS_ACCESS_KEY_ID\`: [Prod AWS Access Key]
- \`AWS_SECRET_ACCESS_KEY\`: [Prod AWS Secret Key]
- \`AWS_REGION\`: [Prod AWS Region]
- \`EKS_CLUSTER_NAME\`: [Prod EKS Cluster Name]

## Supported Deployment Targets
- \`aws-ck-qa-demo\` - AWS cluster, demo namespace
- \`aws-ck-qa-codekarma\` - AWS cluster, codekarma namespace
- \`aws-prod-codekarma\` - AWS cluster, codekarma namespace
EOF

    if [ -n "$GCP_PROJECT_ID" ] && [ -n "$GKE_CLUSTER_NAME" ] && [ -n "$GCP_REGION" ]; then
        cat >> github-actions-config.md << EOF
- \`gcp-dev-demo\` - GCP cluster, demo namespace
- \`gcp-dev-codekarma\` - GCP cluster, codekarma namespace
- \`gcp-prod-codekarma\` - GCP cluster, codekarma namespace
EOF
    fi

    cat >> github-actions-config.md << EOF

## Usage Examples
\`\`\`bash
# Deploy to single cluster
Environment: demo
Deploy Targets: aws-ck-qa-demo

# Deploy to multiple clusters
Environment: prod
Deploy Targets: aws-ck-qa-codekarma,aws-prod-codekarma

# Deploy to AWS and GCP clusters
Environment: dev
Deploy Targets: aws-ck-qa-demo,gcp-dev-demo
\`\`\`

## Environment Setup Instructions
1. Go to: https://github.com/$REPO_NAME/settings/environments
2. Create environments: \`demo\`, \`dev\`, \`prod\`
3. For each environment, add the same secret names but with environment-specific values
4. The workflow will automatically use the correct secrets based on the selected environment

## Verification Commands
\`\`\`bash
# Check deployment status
kubectl get pods -n demo
kubectl get pods -n codekarma

# Check Helm releases
helm list -n demo
helm list -n codekarma

# Check logs
kubectl logs -f deployment/ck-intel-collector -n demo
\`\`\`
EOF
    
    print_status "Configuration summary saved to github-actions-config.md"
}

# Main execution
main() {
    echo ""
    print_info "Starting GitHub Actions setup..."
    echo ""
    
    check_requirements
    get_aws_config
#    get_gcp_config
    get_github_config
    generate_aws_credentials
    create_github_secrets
    create_github_environments
    test_aws_connectivity
    generate_summary
    
    echo ""
    print_status "Setup completed successfully!"
    echo ""
    print_info "Next steps:"
    echo "  1. Review the configuration in github-actions-config.md"
    echo "  2. Push your code to trigger the workflow"
    echo "  3. Monitor the deployment in GitHub Actions"
    echo ""
}

# Run main function
main "$@" 