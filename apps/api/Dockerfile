FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /githarbour ./cmd/api
RUN CGO_ENABLED=0 go build -o /migrate ./cmd/migrate
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /githarbour /githarbour
COPY --from=build /migrate /migrate
COPY --from=build /src/migrations /migrations
WORKDIR /
ENV PORT=8080
ENV MIGRATIONS_DIR=/migrations
EXPOSE 8080
ENTRYPOINT ["/githarbour"]
