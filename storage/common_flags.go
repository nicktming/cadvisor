package storage

import (
	"flag"
	"time"
)

var ArgDbUsername = flag.String("storage_driver_user", "root", "database username")
var ArgDbPassword = flag.String("storage_driver_password", "root", "database password")
var ArgDbHost = flag.String("storage_driver_host", "localhost:8086", "database host:port")
var ArgDbName = flag.String("storage_driver_db", "cadvisor", "database name")
var ArgDbTable = flag.String("storage_driver_table", "stats", "table name")
var ArgDbIsSecure = flag.Bool("storage_driver_secure", false, "use secure connection with database")
var ArgDbBufferDuration = flag.Duration("storage_driver_buffer_duration", 60*time.Second, "Writes in the storage driver will be buffered for this duration, and committed to the non memory backends as a single transaction")

