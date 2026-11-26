FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /hadir ./cmd/server

FROM alpine:3.20
RUN adduser -D app && mkdir /app/uploads -p && chown app /app/uploads
WORKDIR /app
COPY --from=build /hadir /app/hadir
USER app
ENV PORT=8080
EXPOSE 8080
CMD ["/app/hadir"]
