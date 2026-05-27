FROM golang:1.25.0-alpine3.20

WORKDIR /app

RUN apk add --no-cache git build-base

COPY . .
RUN go build -o MediaSaveBot

CMD ["./MediaSaveBot"]
