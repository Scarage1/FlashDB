// flashdb-benchmark - Benchmark tool for FlashDB
//
// Usage:
//
//	flashdb-benchmark [flags]
//
// Flags:
//
//	-addr string     Server address (default "localhost:6379")
//	-clients int     Number of parallel clients (default 50)
//	-requests int    Total number of requests (default 100000)
//	-pipeline int    Pipeline commands (default 1)
//	-test string     Test type: set,get,mixed,incr (default "mixed")
package main

import (
	"flag"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flashdb/flashdb/internal/protocol"
)

func main() {
	addr := flag.String("addr", "localhost:6379", "Server address")
	clients := flag.Int("clients", 50, "Number of parallel clients")
	requests := flag.Int("requests", 100000, "Total number of requests")
	testType := flag.String("test", "mixed", "Test type: set,get,mixed,incr")
	flag.Parse()

	fmt.Println("====== FlashDB Benchmark ======")
	fmt.Printf("Server: %s\n", *addr)
	fmt.Printf("Clients: %d\n", *clients)
	fmt.Printf("Requests: %d\n", *requests)
	fmt.Printf("Test: %s\n", *testType)
	fmt.Println()

	var completed int64
	var errors int64
	reqPerClient := *requests / *clients

	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", *addr)
			if err != nil {
				atomic.AddInt64(&errors, int64(reqPerClient))
				return
			}
			defer conn.Close()

			writer := protocol.NewWriter(conn)
			reader := protocol.NewReader(conn)

			for j := 0; j < reqPerClient; j++ {
				key := fmt.Sprintf("key:%d:%d", clientID, j)
				value := fmt.Sprintf("value:%d:%d", clientID, j)

				var cmd [][]byte
				switch *testType {
				case "set":
					cmd = [][]byte{[]byte("SET"), []byte(key), []byte(value)}
				case "get":
					cmd = [][]byte{[]byte("GET"), []byte(key)}
				case "mixed":
					if j%2 == 0 {
						cmd = [][]byte{[]byte("SET"), []byte(key), []byte(value)}
					} else {
						cmd = [][]byte{[]byte("GET"), []byte(key)}
					}
				case "incr":
					cmd = [][]byte{[]byte("INCR"), []byte(fmt.Sprintf("counter:%d", clientID))}
				default:
					cmd = [][]byte{[]byte("PING")}
				}

				if err := writer.WriteArray(cmd); err != nil {
					atomic.AddInt64(&errors, 1)
					continue
				}

				if _, err := reader.ReadValue(); err != nil {
					atomic.AddInt64(&errors, 1)
					continue
				}

				atomic.AddInt64(&completed, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Println("====== Results ======")
	fmt.Printf("Total time: %v\n", elapsed)
	fmt.Printf("Completed: %d\n", completed)
	fmt.Printf("Errors: %d\n", errors)
	fmt.Printf("Requests/sec: %.2f\n", float64(completed)/elapsed.Seconds())
	fmt.Printf("Avg latency: %.3f ms\n", float64(elapsed.Milliseconds())/float64(completed)*float64(*clients))
}
