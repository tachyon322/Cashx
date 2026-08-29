FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /cashx-api ./cmd/api && go build -o /cashx-redirect ./cmd/redirect && go build -o /cashx-worker ./cmd/worker && go build -o /cashx-migrate ./cmd/migrate && go build -o /cashx-seed-kazik ./cmd/seed-kazik && go build -o /cashx-migrate-kazik ./cmd/migrate-kazik

FROM golang:1.26-alpine
COPY --from=build /cashx-api /usr/local/bin/cashx-api
COPY --from=build /cashx-redirect /usr/local/bin/cashx-redirect
COPY --from=build /cashx-worker /usr/local/bin/cashx-worker
COPY --from=build /cashx-migrate /usr/local/bin/cashx-migrate
COPY --from=build /cashx-seed-kazik /usr/local/bin/cashx-seed-kazik
COPY --from=build /cashx-migrate-kazik /usr/local/bin/cashx-migrate-kazik
COPY --from=build /app/migrations /app/migrations
WORKDIR /app
CMD ["cashx-api"]
