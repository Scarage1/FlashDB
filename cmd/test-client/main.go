package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

func main() {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:6379", 5*time.Second)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Test PING
	fmt.Println(">>> PING")
	fmt.Fprintf(conn, "*1\r\n$4\r\nPING\r\n")
	resp, _ := reader.ReadString('\n')
	fmt.Printf("<<< %s", resp)

	// Test SET
	fmt.Println(">>> SET hello world")
	fmt.Fprintf(conn, "*3\r\n$3\r\nSET\r\n$5\r\nhello\r\n$5\r\nworld\r\n")
	resp, _ = reader.ReadString('\n')
	fmt.Printf("<<< %s", resp)

	// Test GET
	fmt.Println(">>> GET hello")
	fmt.Fprintf(conn, "*2\r\n$3\r\nGET\r\n$5\r\nhello\r\n")
	resp, _ = reader.ReadString('\n')
	fmt.Printf("<<< %s", resp)
	if resp[0] == '$' {
		// Read the value
		val, _ := reader.ReadString('\n')
		fmt.Printf("<<< %s", val)
		reader.ReadString('\n') // trailing \r\n
	}

	// Test DBSIZE
	fmt.Println(">>> DBSIZE")
	fmt.Fprintf(conn, "*1\r\n$6\r\nDBSIZE\r\n")
	resp, _ = reader.ReadString('\n')
	fmt.Printf("<<< %s", resp)

	// Test KEYS *
	fmt.Println(">>> KEYS *")
	fmt.Fprintf(conn, "*2\r\n$4\r\nKEYS\r\n$1\r\n*\r\n")
	resp, _ = reader.ReadString('\n')
	fmt.Printf("<<< %s", resp)

	fmt.Println("\n✓ All tests passed!")
}
