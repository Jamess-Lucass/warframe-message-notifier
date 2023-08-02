FROM golang:1.20-alpine AS builder

WORKDIR /usr/src/app

COPY go.mod go.sum .

RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /usr/local/bin/main ./cmd

FROM alpine:3.18

COPY --from=builder /usr/local/bin/main /usr/local/bin/main

ARG API_BASE_URL

ENV API_BASE_URL ${API_BASE_URL}
ENV WF_EE_LOG_FILE_PATH /tmp/warframe/EE.log

CMD ["/usr/local/bin/main"]