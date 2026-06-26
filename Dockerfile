FROM golang:1.23-alpine AS base

RUN apk add make bash git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN make build

CMD ["./gosocial", "web"]
