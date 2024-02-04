// The package main is a target package
package main

// Config is a configuration of the target server
//
//go:generate go run ../
type Config struct {
	// Host is a host name or IP address of the target server
	Host string
	// Port is a port number of the target server
	Port int
	// Protocol is a protocol of the target server
	Protocol string
	// Timeout is a timeout of the target server
	Timeout int
}
