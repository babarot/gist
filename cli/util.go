package cli

import "log"

func ErrorLog(err error) {
	log.Printf("%v\n", err)
}
