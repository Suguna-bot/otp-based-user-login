FROM golang:1.23 AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /app/server /app/server
EXPOSE 8080
ENTRYPOINT ["/app/server"]
