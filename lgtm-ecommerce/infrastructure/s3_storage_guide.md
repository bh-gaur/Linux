# Production Deployment: Configuring Loki, Mimir & Tempo to use AWS S3 Storage

This guide explains how to migrate storage configurations from local filesystem storage (used during development) to AWS S3 or any compatible object store (like MinIO or GCS) for production long-term data retention.

---

## ☁️ 1. Required S3 Buckets
Create three private S3 buckets inside your AWS region:
* `loki-storage-prod` (Loki chunks and TSDB indexes)
* `mimir-storage-prod` (Mimir metrics blocks and rules)
* `tempo-storage-prod` (Tempo trace logs)

---

## 🛠️ 2. Configuration Replacements

### Loki Config (`loki/loki-config.yaml`)
To connect Loki to S3, replace the `common` and `schema_config` storage blocks:
```yaml
common:
  storage:
    s3:
      endpoint: s3.us-east-1.amazonaws.com
      bucketnames: loki-storage-prod
      access_key_id: ${AWS_ACCESS_KEY_ID}
      secret_access_key: ${AWS_SECRET_ACCESS_KEY}
      s3forcepathstyle: false # Set to true if using MinIO
      insecure: false

schema_config:
  configs:
    - from: 2020-10-24
      store: tsdb
      object_store: s3
      schema: v13
      index:
        prefix: index_
        period: 24h
```

---

### Mimir Config (`mimir/mimir-config.yaml`)
To store Prometheus metrics in S3, change the storage backends:
```yaml
common:
  storage:
    backend: s3
    s3:
      endpoint: s3.us-east-1.amazonaws.com
      access_key_id: ${AWS_ACCESS_KEY_ID}
      secret_access_key: ${AWS_SECRET_ACCESS_KEY}
      insecure: false
      bucket_name: mimir-storage-prod

blocks_storage:
  backend: s3
  s3:
    endpoint: s3.us-east-1.amazonaws.com
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
    insecure: false
    bucket_name: mimir-storage-prod

ruler_storage:
  backend: s3
  s3:
    endpoint: s3.us-east-1.amazonaws.com
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
    insecure: false
    bucket_name: mimir-storage-prod
```

---

### Tempo Config (`tempo/tempo-config.yaml`)
Replace the trace storage block:
```yaml
storage:
  trace:
    backend: s3
    wal:
      path: /tmp/tempo/wal
    s3:
      bucket: tempo-storage-prod
      endpoint: s3.us-east-1.amazonaws.com
      access_key: ${AWS_ACCESS_KEY_ID}
      secret_key: ${AWS_SECRET_ACCESS_KEY}
      insecure: false
      forcepathstyle: false
```
