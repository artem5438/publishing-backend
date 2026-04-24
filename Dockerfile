# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/server ./server
COPY --from=builder /src/static ./static
COPY --from=builder /src/templates ./templates
COPY --from=builder /src/docs ./docs
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["./server"]
