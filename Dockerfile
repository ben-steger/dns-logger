
FROM ubuntu:latest

RUN apt-get update && apt-get install python3 python3-pip git -y
RUN DEBIAN_FRONTEND=noninteractive apt-get install golang -y

COPY dnsServer.go /dnsServer.go

WORKDIR /

RUN mkdir /www
RUN touch /www/index.html

RUN go get github.com/google/gopacket
RUN go get github.com/mattn/go-sqlite3
RUN go build dnsServer.go

# RUN nohup go run dnsServer.go &
# RUN cd www && nohup python -m SimpleHTTPServer 901 &

EXPOSE 8090
EXPOSE 80
CMD ["./dnsServer"]