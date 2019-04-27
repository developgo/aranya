/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"arhat.dev/aranya/pkg/connectivity"
)

func getArhatLogs(options *connectivity.LogOptions, stdout, stderr io.WriteCloser) *connectivity.Error {
	// TODO: implement
	return nil
}

func portForwardToHost(protocol string, port int32, in io.Reader, out io.WriteCloser) *connectivity.Error {
	var plainErr error
	conn, plainErr := net.Dial(protocol, fmt.Sprintf("%s:%s", "localhost", strconv.FormatInt(int64(port), 10)))
	if plainErr != nil {
		log.Printf("failed to dial target: %v", plainErr)
		return connectivity.NewCommonError(plainErr.Error())
	}
	defer func() { _ = conn.Close() }()

	go func() {
		log.Printf("starting write routine")
		defer log.Printf("finished write routine")

		if _, err := io.Copy(conn, in); err != nil {
			log.Printf("exception happened in write routine: %v", err)
			return
		}
	}()

	log.Printf("starting read routine")
	defer log.Printf("finished write routine")

	if _, err := io.Copy(out, conn); err != nil {
		log.Printf("exception happened in read routine: %v", err)
	}

	return nil
}
