package installer

import "errors"

func linuxCodeTemporaryIdentity(string) (string, error) {
	return "", errors.New("Linux Code temporary inputs require a POSIX host")
}

func readLinuxCodeDatabase(string) ([]byte, error) {
	return nil, errors.New("Linux Code temporary inputs require a POSIX host")
}
