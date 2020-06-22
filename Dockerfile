FROM golang:1.14

WORKDIR /app

COPY . ./

RUN go get
