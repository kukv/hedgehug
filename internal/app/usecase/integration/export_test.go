package integration

// AutoLinks lets autolink_test.go call autoLinks. That test needs usecasetest,
// which imports replication, which imports this package: it cannot live in
// package integration without an import cycle.
var AutoLinks = autoLinks
