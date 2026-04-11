# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=arm64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /logvalet ./cmd/logvalet/

FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=builder /logvalet /logvalet
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/logvalet"]
CMD ["mcp", "--host", "0.0.0.0"]
