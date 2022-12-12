FROM golang:1.19.3
WORKDIR /app
COPY main.go /app/main.go

RUN CGO_ENABLED=0 go build -o /usr/bin/app /app/main.go

CMD ["/usr/bin/app"]
