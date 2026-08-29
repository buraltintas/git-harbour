FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /githarbour ./cmd/api
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /githarbour /githarbour
COPY --from=build /src/migrations /migrations
WORKDIR /
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/githarbour"]
