package main

// Derived from here: https://medium.com/@openmohan/dns-basics-and-building-simple-dns-server-in-go-6cb8e1cfe461

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

func main() {
	db, err := sql.Open("sqlite3", "./lookups.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlStmt := `
	create table if not exists lookups (id integer not null primary key, time text, domain text, ip text);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}

	writeToHTML() 		// Keep the html file up to date
	setUpRecordTable() // Get the record table setup if it isn't already

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
		serveDNS(u, clientAddr, tcp, db)
	}
}

func serveDNS(u *net.UDPConn, clientAddr net.Addr, request *layers.DNS, db *sql.DB) {
	// Prep the database conneciton
	// db, err := sql.Open("sqlite3", "./lookups.db")
	// fmt.Println("%T", db)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer db.Close()

	replyMess := request
	var dnsAnswer layers.DNSResourceRecord
	dnsAnswer.Type = layers.DNSTypeA

	if(request == nil) {	// 0 is the default of uint16 which is the value of layers.DNSClass types
		fmt.Println("(-) Received malformed DNS lookup")
		return	// Don't respond to lookup, it was malformed
	}
	var err error
	var ip string
	var ok bool
	// ip, ok = records[string(request.Questions[0].Name)]

	row := db.QueryRow("select ip from records where domain = ?", (string(request.Questions[0].Name)))
	// defer rows.Close()

	err = row.Scan(&ip)
	if err != nil {
		fmt.Println(err)
	}

	if !ok {
		//Todo: Log no data present for the IP and handle:todo
	}
	a, _, _ := net.ParseCIDR(ip + "/24")
	dnsAnswer.Type = layers.DNSTypeA
	dnsAnswer.IP = a
	dnsAnswer.Name = []byte(request.Questions[0].Name)
	
	fmt.Println("(+) Received DNS lookup for: " + string(request.Questions[0].Name))
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
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("INSERT INTO lookups (time, domain, ip) VALUES (?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(time.Now(), string(request.Questions[0].Name), clientAddr.String())
	if err != nil {
		log.Fatal(err)
	}
	tx.Commit()
	writeToHTML()
}

func writeToHTML() {
	// Write to the html file
	f, err := os.Create("www/index.html")

	f.WriteString("<!DOCTYPE html><html><head><style>table {  font-family: arial, sans-serif;  border-collapse: collapse;  width: 100%;}td, th {  border: 1px solid #dddddd;  text-align: left;  padding: 8px;}tr:nth-child(even) {  background-color: #dddddd;}</style></head><body><h2>DNS Logger</h2><table>")

	db, err := sql.Open("sqlite3", "./lookups.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("select domain, time, ip from lookups order by id DESC LIMIT 100")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var rowCount int
	rowCount = 0
	for rows.Next() {
		var time string
		var domain string
		var ip string
		err = rows.Scan(&domain, &time, &ip)
		if err != nil {
			log.Fatal(err)
		}

		if err != nil {
			fmt.Println(err)
			return
		}
		l, err := f.WriteString("<tr>" + "<td>" + html.EscapeString(domain) + "</td><td>" + html.EscapeString(time) + "</td><td>" + html.EscapeString(ip) + "</td>" + "</tr>\n")
		if (err != nil || l < 1) {
			fmt.Println(err)
			f.Close()
			return
		}
		rowCount += 1
	}
	f.WriteString("</table></body></html>")
	fmt.Printf("(i) Wrote %d queries to wwww/index.html\n", rowCount)
}

func setUpRecordTable() {
	db, err := sql.Open("sqlite3", "./lookups.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlStmt := `
	create table if not exists records (id integer not null primary key, domain text, ip text);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}

	records := map[string]string{
		"google.com": "1.1.1.1", // Bogus record example (returns 0.0.0.0 otherwise)
		"meta.example.com": "169.254.169.254", // Bogus record example (returns 0.0.0.0 otherwise)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	for domain, ip := range records {
		stmt, err := tx.Prepare("INSERT INTO records (domain, ip) VALUES (?, ?)")
		if err != nil {
			log.Fatal(err)
		}
		defer stmt.Close()
		_, err = stmt.Exec(domain, ip)
		if err != nil {
			log.Fatal(err)
		}
	}
	tx.Commit()
}
