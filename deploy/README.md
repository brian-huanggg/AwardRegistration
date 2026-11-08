# deploy/

Frontend Helm chart (F0-05). The cluster deploys a copy of `frontend/` from the GitOps
repository `platform-gitops` (`apps/esg/frontend`); keep the two in sync. The full procedure is in
[`docs/deployment/`](../docs/deployment/README.md).

- Ingress is disabled by default. The real Ingress (`/` → `esg-frontend`, `/api` → backend,
  6 MiB, TLS) belongs to O0-08.
- `image.repository` is `harbor.pf.internal/lab/esg-frontend`; `image.tag` is required (git SHA).
- Render: `helm template deploy/frontend -f deploy/frontend/values-dev.yaml --set image.tag=abc123`
