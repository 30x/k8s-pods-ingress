/*
Copyright © 2016 Apigee Corporation

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

package nginx

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

func shellOut(cmd string, exitOnFailure bool) {
	// If we are running outside of Kubenetes, KUBE_HOST will be set in which case we do not want to start nginx
	if os.Getenv("KUBE_HOST") != "" {
		return
	}

	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()

	if err != nil {
		msg := fmt.Sprintf("Failed to execute (%v): %v, err: %v", cmd, string(out), err)

		if exitOnFailure {
			log.Fatal(msg)
		} else {
			log.Println(msg)
		}
	}
}

func writeNginxConf(conf string) {
	log.Println(conf)

	// If we are running outside of Kuberenetes, KUBE_HOST will be set in which case we do not want to write nginx.conf
	if os.Getenv("KUBE_HOST") == "" {
		// Create the nginx.conf file based on the template
		if w, err := os.Create(NginxConfPath); err != nil {
			log.Fatalf("Failed to open %s: %v", NginxConfPath, err)
		} else if _, err := io.WriteString(w, conf); err != nil {
			log.Fatalf("Failed to write template %v", err)
		}

		log.Printf("Wrote nginx configuration to %s\n", NginxConfPath)
	}
}

/*
RestartServer restarts nginx using the provided configuration.
*/
func RestartServer(conf string, exitOnFailure bool) {
	log.Println("Reloading nginx with the following configuration:")

	writeNginxConf(conf)

	log.Println("Restarting nginx")

	shellOut("nginx -s reload", exitOnFailure)
}

/*
StartServer starts nginx using the provided configuration.
*/
func StartServer(conf string) {
	log.Println("Starting nginx with the following configuration:")

	writeNginxConf(conf)

	log.Println("Starting nginx")

	shellOut("nginx", true)
}
