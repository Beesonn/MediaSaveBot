FROM golang:alpine

WORKDIR /app

RUN apk add --no-cache git build-base

COPY . .
RUN go build -o MediaSaveBot

CMD ["./MediaSaveBot"]
