package health

// Result wraps the Check response.
// Status can always be used to reflect the current service status, and Err contains diagnostic error information.
type Result struct {
	// Status is the final resolved status after querying the Checkers.
	Status Status
	// Err are any diagnostic errors encountered while determining status.
	Err error
}
