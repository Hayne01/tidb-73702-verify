// Smoke reproducer for pingcap/tidb#73702. Opens several FRESH connections and
// runs the UNION over information_schema.variables_info as the first statement
// on each, exactly like the issue. On an unpatched server the concurrent lazy
// population of SessionVars.systems kills the whole process; the client then
// sees lost connections. Run inside the tidb module so it uses the vendored
// go-sql-driver/mysql dependency.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	port := "4000"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	dsn := "root@tcp(127.0.0.1:" + port + ")/?timeout=5s&readTimeout=15s&writeTimeout=15s"

	// Wait for the server to accept connections.
	var db *sql.DB
	for i := 0; i < 90; i++ {
		var err error
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				break
			}
			db.Close()
		}
		time.Sleep(time.Second)
	}
	if db != nil {
		db.Close()
	}

	const query = "SELECT 1 FROM information_schema.variables_info UNION ALL SELECT 1 FROM information_schema.variables_info"
	ok, failed := 0, 0
	for i := 0; i < 6; i++ {
		// A brand-new connection pool with a single connection => a fresh session
		// whose systems map has not been populated yet.
		c, err := sql.Open("mysql", dsn)
		if err != nil {
			fmt.Printf("conn %d: open error: %v\n", i, err)
			failed++
			continue
		}
		c.SetMaxOpenConns(1)
		rows, err := c.Query(query)
		if err != nil {
			fmt.Printf("conn %d: QUERY ERROR: %v\n", i, err)
			failed++
			c.Close()
			continue
		}
		n := 0
		for rows.Next() {
			n++
		}
		if err := rows.Err(); err != nil {
			fmt.Printf("conn %d: ROWS ERROR: %v\n", i, err)
			failed++
		} else {
			fmt.Printf("conn %d: ok, %d rows\n", i, n)
			ok++
		}
		rows.Close()
		c.Close()
	}
	fmt.Printf("SMOKE_RESULT ok=%d failed=%d\n", ok, failed)
}