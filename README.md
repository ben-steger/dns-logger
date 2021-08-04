# dns-logger
1. `go get github.com/google/gopacket`
2. `go get github.com/mattn/go-sqlite3`
3. `mkdir www`
4. `go run dnsServer.go`
5. Test via: `nslookup google.com localhost -port=53`
