# Postman collection (local)

Hand-maintained from `docs/API/api-endpoints.md`; `backend/api/openapi.yaml` wins on schemas once complete.

1. Import `ESG-Idea-Contest.postman_collection.json` and `local.postman_environment.json`.
2. Select the **ESG local** environment; fill `userPassword`, `adminEmpNo`, `adminPassword` (keep secrets in the *current value* column, never commit them).
3. `baseUrl` defaults to `http://localhost:8080/api/v1`; change it to match the server port.
4. Run `auth / POST register`, then `POST login (User)`. A collection pre-request script fetches `GET /auth/csrf` and adds `X-CSRF-Token` to every write; the token is dropped after login/logout and on `CSRF_INVALID`.
5. Multipart requests (create idea, replace slides): choose a PDF/PPTX in the `slides` field. The filename must equal `expectedFilenameBase` + extension.

Note: the session cookie is `__Host-session` (Secure). Over plain `http://localhost` Postman may refuse to store it; if so, serve over HTTPS or check the server's local cookie setting.
Operations endpoints (`/actuator/*`) are on a separate management port and not included.
