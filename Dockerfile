FROM golang:alpine as build
RUN apk --no-cache add gcc g++ make git
WORKDIR /go/src/app
COPY . .
# RUN go mod init webserver
# RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o web-app

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /usr/bin
COPY --from=build /go/src/app go/app
WORKDIR /usr/bin/go/app
EXPOSE 80
ENTRYPOINT ./web-app --port 80