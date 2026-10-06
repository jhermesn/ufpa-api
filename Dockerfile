# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/ufpa-api .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/ufpa-api /ufpa-api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/ufpa-api"]
