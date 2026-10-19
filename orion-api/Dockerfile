FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /orion-api .

FROM alpine:3.21
RUN adduser -D -u 10001 app
COPY --from=build /orion-api /usr/local/bin/orion-api
USER app
EXPOSE 8080
CMD ["orion-api"]
