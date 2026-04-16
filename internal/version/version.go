// Package version provides centralized version information for the application.
// The Version constant can be overridden at build time using ldflags:
//
//	go build -ldflags "-X github.com/JB-SelfCompany/yggmap/internal/version.Version=x.x.x"
package version

// Version is the application version.
const Version = "0.1.0"
