# OpenTelemetry Helm Chart Version Management

## 🎯 **Why Version Pinning Matters**

Using the latest version of external Helm charts can break your deployment when they make changes. This project now uses **version pinning** for production stability.

## 📌 **Current Configuration**

### Pinned Version
- **Current Version**: `0.126.0`
- **Location**: `Makefile` variable `OTEL_HELM_CHART_VERSION`

### How It Works
```bash
# The deploy-kind target now uses the pinned version:
make deploy-kind
```

## 🔧 **Version Management Commands**

### Check Current Version and Available Updates
```bash
make check-helm-versions
```

**Output:**
```
Current pinned version: 0.126.0

Available versions:
NAME                                    CHART VERSION   APP VERSION     DESCRIPTION
open-telemetry/opentelemetry-collector  0.126.0         0.127.0         OpenTelemetry Collector Helm chart for Kubernetes
open-telemetry/opentelemetry-collector  0.125.0         0.126.0         OpenTelemetry Collector Helm chart for Kubernetes
...
```

## 🚀 **How to Update Versions**

### 1. Check Available Versions
```bash
make check-helm-versions
```

### 2. Test New Version in Development
```bash
# Edit Makefile and change:
OTEL_HELM_CHART_VERSION=0.127.0  # Update to desired version
```

### 3. Test Deployment
```bash
make deploy-kind
```

### 4. Validate Everything Works
- Check that deployment succeeds
- Verify metrics are flowing correctly
- Test all expected functionality

### 5. Commit the Version Update
```bash
git add Makefile
git commit -m "chore: update OpenTelemetry Helm chart to v0.127.0"
```

## 🔄 **Upgrade Strategy**

1. **Monitor**: Use `make check-helm-versions` to check for new releases
2. **Test**: Update version in development environment first
3. **Validate**: Ensure all functionality works with new version
4. **Deploy**: Update production only after thorough testing
5. **Rollback**: Keep previous version number handy for quick rollback

## 📋 **Version Compatibility Matrix**

| Chart Version | App Version | Status | Notes |
|---------------|-------------|---------|-------|
| 0.126.0 | 0.127.0 | ✅ **Current** | Tested & Working |
| 0.125.0 | 0.126.0 | ✅ Compatible | Previous stable |
| 0.124.0 | 0.125.0 | ⚠️ Untested | - |

## 🛡️ **Production Best Practices**

1. **Always pin versions** in production environments
2. **Test upgrades** in development first
3. **Monitor release notes** for breaking changes
4. **Keep rollback plan** ready
5. **Document compatibility** issues found during testing

This approach ensures your deployments remain stable and predictable! 🎯 