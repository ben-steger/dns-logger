package main

// Found here: https://medium.com/@openmohan/dns-basics-and-building-simple-dns-server-in-go-6cb8e1cfe461

import (
	"fmt"
	"net"
	"os"

	"github.com/google/gopacket"
	layers "github.com/google/gopacket/layers"
	"database/sql"
    _ "github.com/mattn/go-sqlite3"
	"time"
	"log"
	"html"
)

var records map[string]string

func main() {
	// Initialize the database if this hasn't been done already
	// database, err := sql.Open("sqlite3", "./foo.db")
	// if err != nil {
	// 	log.Fatal(err)
	// }
    // statement, _ := database.Prepare("CREATE TABLE IF NOT EXISTS dns (id INTEGER PRIMARY KEY, time TEXT, domain_lookup TEXT)")
    // statement.Exec()

	records = map[string]string{
		"meta.praetorain.com": "169.254.169.254",
	}

	db, err := sql.Open("sqlite3", "./lookups.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlStmt := `
	create table if not exists lookups (id integer not null primary key, time text, domain text);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}

	writeToHTML() // Keep the html file up to date

	//Listen on UDP Port
	addr := net.UDPAddr{
		Port: 53,
		IP:   net.ParseIP("0.0.0.0"),
	}
	u, _ := net.ListenUDP("udp", &addr)

	// Wait to get request on that port
	for {
		tmp := make([]byte, 1024)
		_, addr, _ := u.ReadFrom(tmp)
		clientAddr := addr
		packet := gopacket.NewPacket(tmp, layers.LayerTypeDNS, gopacket.Default)
		dnsPacket := packet.Layer(layers.LayerTypeDNS)
		tcp, _ := dnsPacket.(*layers.DNS)
		serveDNS(u, clientAddr, tcp)
	}
}

func serveDNS(u *net.UDPConn, clientAddr net.Addr, request *layers.DNS) {
	replyMess := request
	var dnsAnswer layers.DNSResourceRecord
	dnsAnswer.Type = layers.DNSTypeA
	var ip string
	var err error
	var ok bool
	ip, ok = records[string(request.Questions[0].Name)]
	if !ok {
		//Todo: Log no data present for the IP and handle:todo
	}
	a, _, _ := net.ParseCIDR(ip + "/24")
	dnsAnswer.Type = layers.DNSTypeA
	dnsAnswer.IP = a
	dnsAnswer.Name = []byte(request.Questions[0].Name)
	fmt.Println(string(request.Questions[0].Name))
	dnsAnswer.Class = layers.DNSClassIN
	replyMess.QR = true
	replyMess.ANCount = 1
	replyMess.OpCode = layers.DNSOpCodeNotify
	replyMess.AA = true
	replyMess.Answers = append(replyMess.Answers, dnsAnswer)
	replyMess.ResponseCode = layers.DNSResponseCodeNoErr
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{} // See SerializeOptions for more details.
	err = replyMess.SerializeTo(buf, opts)
	if err != nil {
		panic(err)
	}
	u.WriteTo(buf.Bytes(), clientAddr)

	// Store the lookup in the db
	db, err := sql.Open("sqlite3", "./lookups.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("INSERT INTO lookups (time, domain) VALUES (?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(time.Now(), string(request.Questions[0].Name))
	if err != nil {
		log.Fatal(err)
	}
	tx.Commit()
	writeToHTML()
}

func writeToHTML() {
	// Write to the html file
	f, err := os.Create("www/index.html")

	db, err := sql.Open("sqlite3", "./lookups.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("select domain, time from lookups order by id DESC LIMIT 100")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var time string
		var domain string
		err = rows.Scan(&domain, &time)
		if err != nil {
			log.Fatal(err)
		}

		if err != nil {
			fmt.Println(err)
			return
		}
		l, err := f.WriteString("<li>" + html.EscapeString(domain) + "---------->" + html.EscapeString(time) +"</li>\n")
		fmt.Printf("wrote %d bytes\n", l)
		if err != nil {
			fmt.Println(err)
			f.Close()
			return
		}

	}
}

// func addRecord(database *database, domain *string) {

// }