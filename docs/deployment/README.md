# Deployment manual: frontend on K3S (DEV)

How the frontend image is built, pushed to Harbor and deployed to the K3S cluster through Argo CD.
Scope is the frontend only (F0-05). The backend chart, the Flyway PreSync Job and the real
Ingress are O0-06 to O0-08; the automated pipeline is O0-09. Until O0-09 exists, steps 2 to 4 are
manual.

## 1. How it fits together

```
this repo                         Harbor                     platform-gitops (GitLab)          K3S
frontend/Dockerfile  --build-->   lab/esg-frontend:<sha>     apps/esg/frontend/values-dev.yaml
deploy/frontend (chart source)                                 image.tag: <sha>   --Argo CD-->  ns esg
                                                              registry/esg-*.yaml
```

| Piece | Location |
|---|---|
| Image | `harbor.pf.internal/lab/esg-frontend:<git SHA>` (shared Harbor project `lab`) |
| Chart source | `deploy/frontend/` in this repo |
| Deployed chart | `apps/esg/frontend/` in `platform-gitops` (a copy of the chart plus `values-dev.yaml`) |
| Argo CD apps | `esg-secrets` (sync-wave 3), `esg-frontend` (sync-wave 4), both in namespace `esg` |
| Pull secret | `harbor-pull`, built by External Secrets from Vault `secret/k3s/harbor-pull` |

The cluster never receives a push credential. It holds only the pull token.

## 2. Build and test the image locally

The build context is the repository root, because `tsc` also type-checks tests that import `../testdata`.

```bash
scripts/test-frontend-image.sh        # builds esg-frontend:test and checks nginx behaviour
```

The script checks SPA fallback, cache headers, CSP and other security headers, the JSON 404 under
`/api/`, `/healthz`, a read-only root filesystem and a non-root user. It must print
`frontend image OK`.

## 3. Push the image to Harbor

Use a **push** robot account of project `lab`. Do not reuse the pull credential.

```bash
SHA=$(git rev-parse --short=12 HEAD)
IMG=harbor.pf.internal/lab/esg-frontend

docker build -f frontend/Dockerfile -t "$IMG:$SHA" .
docker login harbor.pf.internal          # robot name and secret; read the secret from stdin
docker push "$IMG:$SHA"
```

Rules:

- The tag is the git SHA of the commit that was built. Never push `latest`.
- Build from a clean working tree, so the SHA identifies the content.
- Harbor uses an internal CA. If `docker login` reports an unknown authority, install the CA
  certificate (`ansible/files/vault-ca.crt` in the infrastructure repository) in the Docker host.

## 4. Release the tag through GitOps

In `platform-gitops`, set the tag in `apps/esg/frontend/values-dev.yaml`:

```yaml
image:
  tag: "<SHA from step 3>"
```

```bash
git switch -c chore/esg-frontend-<SHA>
git commit -am "chore(esg): deploy frontend <SHA> to dev"
git switch main && git merge --no-ff chore/esg-frontend-<SHA> && git push origin main
```

Argo CD polls the repository (about every 3 minutes) and syncs with automated prune and self-heal.
To sync immediately: `kubectl -n argocd annotate app esg-frontend argocd.argoproj.io/refresh=hard --overwrite`.

An empty `image.tag` makes the chart fail on purpose (`image.tag is required (git SHA)`), so the
application shows `Unknown` until a tag is committed.

## 5. Verify

Run on a host that has cluster access (`devbox`):

```bash
kubectl -n argocd get app root esg-secrets esg-frontend     # all Synced / Healthy
kubectl -n esg get secretstore,externalsecret,secret         # store Valid, harbor-pull SecretSynced
kubectl -n esg get deploy,pod -l app.kubernetes.io/name=esg-frontend
kubectl -n esg rollout status deploy/esg-frontend
```

Expected: 2 ready Pods, a PodDisruptionBudget, and a `harbor-pull` Secret of type
`kubernetes.io/dockerconfigjson`.

Application check without an Ingress:

```bash
kubectl -n esg port-forward svc/esg-frontend 8080:80 &
curl -sI localhost:8080/healthz          # 200
curl -s  localhost:8080/ | grep 'id="root"'
```

The F0-05 exit criterion (deadline banner visible on DEV) needs the Ingress and the backend, so it
is completed with O0-08 and O0-09.

## 6. Rollback

Revert the tag commit in `platform-gitops` and push. Argo CD returns to the previous SHA. Do not
use `kubectl rollout undo`: self-heal restores the Git state.

## 7. Enable the Ingress (after O0-08)

Set these in `values-dev.yaml` once the application domain exists (OI-06):

```yaml
ingress:
  enabled: true
  host: <application domain>
```

This Ingress only routes `/`. The combined Ingress for `/` and `/api` (6 MiB limit, TLS) replaces
it in O0-08, so remove this one at that point.

## 8. One-time cluster setup (already done for DEV)

Kept here so the environment can be rebuilt. All paths are in `platform-gitops`.

| Step | What | Where |
|---|---|---|
| 1 | Create the Vault role `eso-esg` and policy `k3s-pull-esg` (read `secret/data/k3s/harbor-pull`) | `vault/k3s-jwt-auth.sh pubkeys.pem pull-only esg`, run from a host that reaches Vault |
| 2 | Store the pull credential: fields `username` and `token` | `vault kv put secret/k3s/harbor-pull ...` (values from stdin, never in the repo) |
| 3 | Grant that pull robot read access to Harbor project `lab` | Harbor UI |
| 4 | Namespace resources: ServiceAccount `eso-vault`, SecretStore `vault`, CA ConfigMap, ExternalSecret `harbor-pull`, metadata-egress NetworkPolicy | `apps/esg/secrets/`, `apps/network-policies/deny-metadata-esg.yaml` |
| 5 | Argo CD applications | `registry/esg-secrets.yaml`, `registry/esg-frontend.yaml` |

## 9. Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `esg-frontend` is `Unknown`, message `image.tag is required` | No tag committed. Do step 4. |
| SecretStore `InvalidProviderConfig`, log `role "eso-esg" could not be found` | Vault role missing. Do setup step 1. |
| ExternalSecret `SecretSyncedError` after the store became Valid | Exponential back-off. Force a retry: `kubectl -n esg annotate externalsecret harbor-pull force-sync="$(date +%s)" --overwrite` |
| Pod `ImagePullBackOff`, `unauthorized` | The pull robot cannot read project `lab`, or the secret is empty. Check setup steps 2 and 3. |
| Pod `ImagePullBackOff`, `not found` | The tag was never pushed, or the repository path differs. Compare `values.yaml` with Harbor. |
| Pod `ImagePullBackOff`, `x509` | The node does not trust the Harbor CA. |
| Vault CLI says `connection refused` or `HTTP response to HTTPS client` | `VAULT_ADDR` is unset, or a local dev Vault on `127.0.0.1:8200` shadows the IAP tunnel. Use the tunnel on another port (for example 18200) and set `VAULT_ADDR=https://localhost:18200`. |
