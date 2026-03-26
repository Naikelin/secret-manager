# Kubernetes Deployment Guide

Complete guide for deploying Secret Manager to Kubernetes using either raw manifests or Helm charts.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start with Helm](#quick-start-with-helm)
- [Manual Deployment with kubectl](#manual-deployment-with-kubectl)
- [Configuration](#configuration)
- [GitOps Integration (FluxCD)](#gitops-integration-fluxcd)
- [Monitoring and Health Checks](#monitoring-and-health-checks)
- [Scaling](#scaling)
- [Backup and Restore](#backup-and-restore)
- [Troubleshooting](#troubleshooting)
- [Uninstall](#uninstall)

---

## Prerequisites

Before deploying Secret Manager to Kubernetes, ensure you have:

### Required Tools

- **kubectl** (v1.24+): Kubernetes CLI
  ```bash
  kubectl version --client
  ```

- **Helm** (v3.8+): Kubernetes package manager (for Helm deployments)
  ```bash
  helm version
  ```

- **Kubernetes Cluster** (v1.24+) with:
  - At least 3 worker nodes (for HA)
  - Storage provisioner (for PostgreSQL persistent volumes)
  - Ingress controller (NGINX recommended)
  - cert-manager (optional, for TLS certificates)

### Cluster Access

Verify cluster access:

```bash
kubectl cluster-info
kubectl get nodes
```

### Storage Class

Verify a storage class exists:

```bash
kubectl get storageclass
```

If you need to set a default storage class:

```bash
kubectl patch storageclass <your-storage-class> -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
```

---

## Quick Start with Helm

The fastest way to deploy Secret Manager is using Helm.

### 1. Install with Default Values

```bash
helm install secret-manager ./helm/secret-manager \
  --namespace secret-manager \
  --create-namespace
```

### 2. Install with Custom Domain

```bash
helm install secret-manager ./helm/secret-manager \
  --namespace secret-manager \
  --create-namespace \
  --set ingress.host=secrets.mycompany.com \
  --set backend.jwtSecret=$(openssl rand -base64 32)
```

### 3. Install for Production

```bash
# Generate secure secrets
JWT_SECRET=$(openssl rand -base64 32)
POSTGRES_PASSWORD=$(openssl rand -base64 32)

# Install with production values
helm install secret-manager ./helm/secret-manager \
  --namespace secret-manager \
  --create-namespace \
  --values ./helm/secret-manager/values-prod.yaml \
  --set backend.jwtSecret=$JWT_SECRET \
  --set backend.azure.clientId=<your-azure-client-id> \
  --set backend.azure.clientSecret=<your-azure-client-secret> \
  --set backend.azure.tenantId=<your-azure-tenant-id> \
  --set backend.git.repoUrl=https://github.com/yourorg/secrets-repo.git \
  --set backend.git.token=<your-git-token> \
  --set backend.sops.ageKey=<your-age-key>
```

### 4. Verify Installation

```bash
# Check Helm release
helm status secret-manager -n secret-manager

# Check pods
kubectl get pods -n secret-manager

# Follow logs
kubectl logs -n secret-manager -l app=backend -f
```

### 5. Access the Application

```bash
# If ingress is configured
echo "Application URL: https://$(kubectl get ingress -n secret-manager -o jsonpath='{.items[0].spec.rules[0].host}')"

# Or use port forwarding
kubectl port-forward -n secret-manager svc/frontend 3000:3000
# Open http://localhost:3000
```

---

## Manual Deployment with kubectl

Deploy using raw Kubernetes manifests for more control.

### 1. Review and Customize Secrets

**IMPORTANT**: Before deploying, update secrets in `k8s/backend/secret.yaml` and `k8s/postgres/secret.yaml`:

```bash
# Generate secure secrets
export JWT_SECRET=$(openssl rand -base64 32)
export POSTGRES_PASSWORD=$(openssl rand -base64 32)

# Update the secrets (or use kubectl create secret)
```

### 2. Deploy Using Script

```bash
./scripts/deploy-k8s.sh
```

### 3. Or Deploy Manually

```bash
# Create namespace
kubectl create namespace secret-manager

# Apply all manifests
kubectl apply -f k8s/ -n secret-manager

# Wait for rollout
kubectl rollout status deployment/backend -n secret-manager
kubectl rollout status deployment/frontend -n secret-manager
```

### 4. Verify Deployment

```bash
# Check all resources
kubectl get all -n secret-manager

# Check persistent volumes
kubectl get pvc -n secret-manager

# Check secrets
kubectl get secrets -n secret-manager
```

---

## Configuration

### Values Customization (Helm)

Create a custom `values-custom.yaml`:

```yaml
# Custom values for my deployment
image:
  backend:
    repository: ghcr.io/myorg/secret-manager-backend
    tag: "v1.2.3"
  frontend:
    repository: ghcr.io/myorg/secret-manager-frontend
    tag: "v1.2.3"

ingress:
  host: secrets.example.com
  tls:
    enabled: true

backend:
  authProvider: azure
  azure:
    clientId: "your-client-id"
    tenantId: "your-tenant-id"
  
  git:
    repoUrl: https://github.com/myorg/secrets-repo.git
    branch: main
  
  sops:
    enabled: true
  
  drift:
    checkInterval: 10m
    webhookUrl: https://hooks.slack.com/services/YOUR/WEBHOOK/URL

postgres:
  storageSize: 50Gi
  storageClass: fast-ssd

resources:
  backend:
    requests:
      memory: "512Mi"
      cpu: "500m"
    limits:
      memory: "1Gi"
      cpu: "1000m"

autoscaling:
  backend:
    enabled: true
    minReplicas: 3
    maxReplicas: 20
```

Install with custom values:

```bash
helm install secret-manager ./helm/secret-manager \
  -f values-custom.yaml \
  --namespace secret-manager \
  --create-namespace
```

### Secrets Management

#### Option 1: Helm Values (Not Recommended for Production)

```bash
helm install secret-manager ./helm/secret-manager \
  --set backend.jwtSecret=<secret> \
  --set backend.azure.clientSecret=<secret>
```

#### Option 2: Kubernetes Secrets (Recommended)

```bash
# Create secrets before Helm install
kubectl create secret generic backend-secrets \
  -n secret-manager \
  --from-literal=JWT_SECRET=$(openssl rand -base64 32) \
  --from-literal=DATABASE_URL=postgres://... \
  --from-literal=AZURE_CLIENT_SECRET=<secret> \
  --from-literal=GIT_TOKEN=<token> \
  --from-literal=SOPS_AGE_KEY=<age-key>

# Then install without setting sensitive values
helm install secret-manager ./helm/secret-manager \
  --namespace secret-manager
```

#### Option 3: Sealed Secrets (Best for GitOps)

```bash
# Install sealed-secrets controller
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm install sealed-secrets sealed-secrets/sealed-secrets -n kube-system

# Create sealed secret
kubectl create secret generic backend-secrets \
  --from-literal=JWT_SECRET=$(openssl rand -base64 32) \
  --dry-run=client -o yaml | \
  kubeseal -o yaml > k8s/backend/sealed-secret.yaml

# Commit sealed-secret.yaml to git (it's encrypted!)
```

#### Option 4: External Secrets Operator

```bash
# Install external-secrets
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets external-secrets/external-secrets -n external-secrets-system --create-namespace

# Create SecretStore and ExternalSecret
kubectl apply -f k8s/backend/external-secret.yaml
```

### Environment-Specific Configurations

#### Development

```yaml
# values-dev.yaml
replicaCount:
  backend: 1
  frontend: 1

postgres:
  storageSize: 5Gi

backend:
  logLevel: debug
  authProvider: mock

autoscaling:
  backend:
    enabled: false
```

#### Staging

```yaml
# values-staging.yaml
replicaCount:
  backend: 2
  frontend: 1

postgres:
  storageSize: 20Gi

backend:
  logLevel: info
  authProvider: azure

autoscaling:
  backend:
    enabled: true
    minReplicas: 2
    maxReplicas: 5
```

#### Production

Use `values-prod.yaml` with appropriate resource limits, replica counts, and autoscaling.

---

## GitOps Integration (FluxCD)

Secret Manager includes FluxCD integration for continuous deployment.

### Existing FluxCD Setup

The project already has FluxCD configuration in `flux-config/`:

```bash
flux-config/
├── gitrepository.yaml    # Git repository source
├── kustomization.yaml    # FluxCD kustomization
└── README.md
```

### Integrate Helm Chart with Flux

Create a `HelmRelease` resource:

```yaml
# flux-config/helm-release.yaml
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: secret-manager
  namespace: flux-system
spec:
  interval: 5m
  chart:
    spec:
      chart: ./helm/secret-manager
      sourceRef:
        kind: GitRepository
        name: secret-manager
        namespace: flux-system
      interval: 1m
  values:
    image:
      backend:
        repository: ghcr.io/yourorg/secret-manager-backend
        tag: "1.0.0"
      frontend:
        repository: ghcr.io/yourorg/secret-manager-frontend
        tag: "1.0.0"
    ingress:
      host: secrets.example.com
  valuesFrom:
    - kind: Secret
      name: secret-manager-values
      valuesKey: values.yaml
```

### Deploy with Flux

```bash
# Bootstrap Flux (if not already done)
flux bootstrap github \
  --owner=<your-org> \
  --repository=<your-repo> \
  --branch=main \
  --path=./flux-config

# Apply HelmRelease
kubectl apply -f flux-config/helm-release.yaml

# Watch reconciliation
flux get helmreleases -n flux-system --watch
```

### Automated Updates with Flux

Enable image automation:

```yaml
# flux-config/image-update-automation.yaml
apiVersion: image.toolkit.fluxcd.io/v1beta1
kind: ImageUpdateAutomation
metadata:
  name: secret-manager
  namespace: flux-system
spec:
  interval: 1m
  sourceRef:
    kind: GitRepository
    name: secret-manager
  git:
    checkout:
      ref:
        branch: main
    commit:
      author:
        email: flux@example.com
        name: Flux Bot
    push:
      branch: main
  update:
    path: ./flux-config
    strategy: Setters
```

---

## Monitoring and Health Checks

### Health Endpoints

Backend exposes health endpoints:

```bash
# Liveness probe
curl http://backend.secret-manager.svc.cluster.local:8080/api/health

# Readiness probe (same endpoint)
curl http://backend.secret-manager.svc.cluster.local:8080/api/health
```

### View Logs

```bash
# Backend logs
kubectl logs -n secret-manager -l app=backend -f

# Frontend logs
kubectl logs -n secret-manager -l app=frontend -f

# PostgreSQL logs
kubectl logs -n secret-manager -l app=postgres -f

# All pods
kubectl logs -n secret-manager --all-containers=true -f
```

### Metrics (Prometheus Integration)

If you have Prometheus installed, create a `ServiceMonitor`:

```yaml
# k8s/backend/servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: backend
  namespace: secret-manager
spec:
  selector:
    matchLabels:
      app: backend
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

### Grafana Dashboard

Import the Secret Manager dashboard (TODO: create dashboard JSON).

### Alerts

Example PrometheusRule:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: secret-manager-alerts
  namespace: secret-manager
spec:
  groups:
  - name: secret-manager
    interval: 30s
    rules:
    - alert: SecretManagerBackendDown
      expr: up{job="backend"} == 0
      for: 5m
      labels:
        severity: critical
      annotations:
        summary: "Secret Manager backend is down"
    
    - alert: SecretManagerDriftDetected
      expr: increase(drift_events_total[5m]) > 0
      labels:
        severity: warning
      annotations:
        summary: "Drift detected in secrets"
```

---

## Scaling

### Manual Scaling

```bash
# Scale backend
kubectl scale deployment backend -n secret-manager --replicas=5

# Scale frontend
kubectl scale deployment frontend -n secret-manager --replicas=3
```

### Horizontal Pod Autoscaler (HPA)

HPA is already configured in `k8s/backend/hpa.yaml` and enabled by default in Helm.

#### View HPA Status

```bash
kubectl get hpa -n secret-manager
```

#### Customize HPA

```yaml
# values-custom.yaml
autoscaling:
  backend:
    enabled: true
    minReplicas: 3
    maxReplicas: 20
    targetCPUUtilizationPercentage: 70
    targetMemoryUtilizationPercentage: 80
```

#### Test Autoscaling

```bash
# Generate load (requires hey or similar tool)
hey -z 2m -c 50 https://secrets.example.com/api/health

# Watch HPA scale up
kubectl get hpa -n secret-manager --watch
```

### Vertical Pod Autoscaler (VPA)

Install VPA:

```bash
git clone https://github.com/kubernetes/autoscaler.git
cd autoscaler/vertical-pod-autoscaler
./hack/vpa-up.sh
```

Create VPA for backend:

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: backend-vpa
  namespace: secret-manager
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: Deployment
    name: backend
  updatePolicy:
    updateMode: "Auto"
```

---

## Backup and Restore

### PostgreSQL Backup Strategies

#### Option 1: Velero (Cluster-Level Backups)

```bash
# Install Velero
velero install --provider aws --bucket my-backup-bucket --secret-file ./credentials-velero

# Backup the namespace
velero backup create secret-manager-backup --include-namespaces secret-manager

# Restore
velero restore create --from-backup secret-manager-backup
```

#### Option 2: pg_dump CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: postgres-backup
  namespace: secret-manager
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: postgres:15-alpine
            env:
            - name: PGHOST
              value: postgres
            - name: PGUSER
              valueFrom:
                secretKeyRef:
                  name: postgres-credentials
                  key: POSTGRES_USER
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-credentials
                  key: POSTGRES_PASSWORD
            command:
            - /bin/sh
            - -c
            - |
              TIMESTAMP=$(date +%Y%m%d_%H%M%S)
              pg_dump -d secretmanager -F c -f /backups/backup_$TIMESTAMP.dump
            volumeMounts:
            - name: backup-storage
              mountPath: /backups
          restartPolicy: OnFailure
          volumes:
          - name: backup-storage
            persistentVolumeClaim:
              claimName: postgres-backup-pvc
```

#### Option 3: Continuous Archiving (WAL)

Configure PostgreSQL for WAL archiving (requires PostgreSQL configuration).

### Restore from Backup

```bash
# Copy backup file to pod
kubectl cp backup_20260326.dump secret-manager/postgres-0:/tmp/

# Restore
kubectl exec -n secret-manager postgres-0 -- pg_restore -d secretmanager -c /tmp/backup_20260326.dump
```

---

## Troubleshooting

### Common Issues

#### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n secret-manager

# Describe pod to see events
kubectl describe pod <pod-name> -n secret-manager

# Check logs
kubectl logs <pod-name> -n secret-manager
```

#### ImagePullBackOff

```bash
# Check image pull secrets
kubectl get secrets -n secret-manager

# Verify image exists
docker pull <image-name>

# Add image pull secret if needed
kubectl create secret docker-registry regcred \
  --docker-server=<your-registry> \
  --docker-username=<username> \
  --docker-password=<password> \
  -n secret-manager
```

#### Database Connection Errors

```bash
# Check PostgreSQL is running
kubectl get pod -n secret-manager -l app=postgres

# Test connection from backend pod
kubectl exec -n secret-manager <backend-pod> -- \
  psql "$DATABASE_URL" -c "SELECT 1"

# Check database logs
kubectl logs -n secret-manager -l app=postgres
```

#### Ingress Not Working

```bash
# Check ingress resource
kubectl get ingress -n secret-manager
kubectl describe ingress -n secret-manager

# Check ingress controller
kubectl get pods -n ingress-nginx

# Check DNS resolution
nslookup secrets.example.com
```

#### TLS Certificate Issues

```bash
# Check certificate
kubectl get certificate -n secret-manager

# Check cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager

# Describe certificate for details
kubectl describe certificate -n secret-manager
```

#### Drift Detection Not Working

```bash
# Check backend logs for drift errors
kubectl logs -n secret-manager -l app=backend | grep drift

# Verify Git credentials
kubectl get secret backend-secrets -n secret-manager -o jsonpath='{.data.GIT_TOKEN}' | base64 -d

# Verify SOPS age key
kubectl get secret backend-secrets -n secret-manager -o jsonpath='{.data.SOPS_AGE_KEY}' | base64 -d

# Test drift detection manually
kubectl exec -n secret-manager <backend-pod> -- curl localhost:8080/api/drift/check
```

### Debug Mode

Enable debug logging:

```bash
# Update ConfigMap
kubectl patch configmap backend-config -n secret-manager \
  -p '{"data":{"LOG_LEVEL":"debug"}}'

# Restart backend pods
kubectl rollout restart deployment backend -n secret-manager
```

### Get Shell in Pod

```bash
# Backend
kubectl exec -it -n secret-manager <backend-pod> -- /bin/sh

# PostgreSQL
kubectl exec -it -n secret-manager postgres-0 -- psql -U admin -d secretmanager
```

### Network Debugging

```bash
# Deploy debug pod
kubectl run debug --image=nicolaka/netshoot -n secret-manager -- sleep infinity

# Test connectivity
kubectl exec -n secret-manager debug -- curl backend:8080/api/health
kubectl exec -n secret-manager debug -- curl frontend:3000
kubectl exec -n secret-manager debug -- nc -zv postgres 5432
```

---

## Uninstall

### Helm Uninstall

```bash
# Uninstall release
helm uninstall secret-manager -n secret-manager

# Delete namespace (removes all resources)
kubectl delete namespace secret-manager
```

### kubectl Uninstall

```bash
# Delete all resources
kubectl delete -f k8s/ -n secret-manager

# Delete namespace
kubectl delete namespace secret-manager
```

### Clean Up Persistent Volumes

**WARNING**: This will delete all data!

```bash
# List PVs
kubectl get pv

# Delete PVs (if not automatically deleted)
kubectl delete pv <pv-name>
```

### Verify Clean Uninstall

```bash
# Check no resources remain
kubectl get all -n secret-manager
kubectl get pvc -n secret-manager
kubectl get secrets -n secret-manager

# Check namespace is gone
kubectl get namespace secret-manager
```

---

## Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Helm Documentation](https://helm.sh/docs/)
- [FluxCD Documentation](https://fluxcd.io/docs/)
- [Secret Manager README](../README.md)
- [Development Guide](../backend/README.md)

---

## Support

For issues and questions:

- GitHub Issues: https://github.com/yourorg/secret-manager/issues
- Documentation: https://docs.secret-manager.io
- Slack: #secret-manager
