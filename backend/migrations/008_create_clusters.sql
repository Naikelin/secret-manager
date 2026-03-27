-- 008_create_clusters.sql
-- Create clusters table for multi-cluster Kubernetes support

CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    kubeconfig_ref VARCHAR(512) NOT NULL,
    environment VARCHAR(50) NOT NULL CHECK (environment IN ('dev', 'staging', 'prod')),
    is_healthy BOOLEAN DEFAULT true,
    last_health_check TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clusters_environment ON clusters(environment);
CREATE INDEX IF NOT EXISTS idx_clusters_health ON clusters(is_healthy);

COMMENT ON TABLE clusters IS 'Kubernetes cluster configurations for multi-cluster support';
COMMENT ON COLUMN clusters.kubeconfig_ref IS 'Path to kubeconfig file or K8s Secret reference (e.g., /etc/kubeconfigs/devops.yaml or secret://secret-manager/kubeconfig-devops)';
