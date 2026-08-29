FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /cashx-api ./cmd/api && go build -o /cashx-redirect ./cmd/redirect && go build -o /cashx-worker ./cmd/worker

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /cashx-api /usr/local/bin/cashx-api
COPY --from=build /cashx-redirect /usr/local/bin/cashx-redirect
COPY --from=build /cashx-worker /usr/local/bin/cashx-worker
WORKDIR /app
CMD ["cashx-api"]
