# Backend image (B0-04 / O0-06). Build context is backend/:
#   docker build -t esg-backend backend
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Static binary on distroless: no shell, uid 65532, works with a read-only root
# filesystem (upload spooling needs a /tmp emptyDir, mounted by the chart).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
# About 75 % of the container memory limit; the chart overrides it together
# with the limit (Backend.md §12).
ENV GOMEMLIMIT=384MiB
EXPOSE 8080 8081
USER 65532:65532
ENTRYPOINT ["/server"]
