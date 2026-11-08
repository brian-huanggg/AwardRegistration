# deploy/

Skeleton Helm chart pieces (F0-05). The authoritative chart lives in the separate
deployment repository (PRD §8.7); `frontend/` here is the frontend part to merge there.

- Ingress (`/` → `esg-frontend`, `/api` → backend, 6 MiB, TLS) belongs to O0-08, not here.
- `image.repository` is a placeholder until OI-02; `image.tag` is required (git SHA).
- Render: `helm template deploy/frontend -f deploy/frontend/values-dev.yaml --set image.tag=abc123`
