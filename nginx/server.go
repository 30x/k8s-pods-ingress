package nginx

import (
	"io"
	"log"
	"os"
	"os/exec"
)

func shellOut(cmd string) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()

	if err != nil {
		log.Fatalf("Failed to execute %v: %v, err: %v", cmd, string(out), err)
	}
}

/*
StartServer starts nginx using the provided configuration or the default configuration.
*/
func StartServer(conf string) {
	nginxConf := conf

	if nginxConf == "" {
		nginxConf = DefaultNginxConf
	}

	log.Println("Starting nginx with the following configuration:")
	log.Println(nginxConf)

	if os.Getenv("KUBE_HOST") == "" {
		// Create the nginx.conf file based on the template
		if w, err := os.Create(NginxConfPath); err != nil {
			log.Fatalf("Failed to open %s: %v", NginxConfPath, err)
		} else if _, err := io.WriteString(w, nginxConf); err != nil {
			log.Fatalf("Failed to write template %v", err)
		}

		log.Printf("Wrote nginx configuration to %s\n", NginxConfPath)

		if conf == "" {
			log.Println("Starting nginx")

			shellOut("nginx")
		} else {
			log.Println("Restarting nginx")

			shellOut("nginx -s reload")
		}
	}
}
