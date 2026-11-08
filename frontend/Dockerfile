# Frontend image (F0-05). Build context is the REPOSITORY ROOT because
# `tsc` also type-checks tests that import ../testdata:
#   docker build -f frontend/Dockerfile -t esg-frontend .
FROM node:22-alpine AS build
WORKDIR /repo/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY testdata /repo/testdata
COPY frontend/ ./
RUN npm run build

# Unprivileged nginx: runs as uid 101, listens on 8080, no root needed.
FROM nginxinc/nginx-unprivileged:1.27-alpine
COPY frontend/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY frontend/nginx/security-headers.conf /etc/nginx/snippets/security-headers.conf
COPY --from=build /repo/frontend/dist /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK NONE
