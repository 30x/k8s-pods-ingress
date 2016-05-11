package utils

/*
IsValidPort returns whether the provided integer is a valid port
*/
func IsValidPort(port int) bool {
	return port > 0 && port < 65536
}
