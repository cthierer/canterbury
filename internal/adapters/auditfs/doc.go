// Package auditfs records audit events as date-rotated JSON Lines files on a
// local filesystem.
//
// The recorder writes outside the vault mirror and treats its configured root as
// sensitive operational data. Each event is marshaled as one JSON object with a
// trailing newline, then appended to the daily file for the event timestamp.
// Short writes are treated as failed audit writes.
package auditfs
