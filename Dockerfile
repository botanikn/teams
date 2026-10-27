FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/task-api ./cmd/api

FROM alpine:3.20
RUN addgroup -S app && adduser -S -G app app
COPY --from=build /out/task-api /usr/local/bin/task-api
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/task-api"]
