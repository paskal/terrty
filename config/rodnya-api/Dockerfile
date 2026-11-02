FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rodnya-api .

FROM scratch
COPY --from=build /rodnya-api /rodnya-api
USER 65534:65534
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD ["/rodnya-api", "--check"]
ENTRYPOINT ["/rodnya-api"]
