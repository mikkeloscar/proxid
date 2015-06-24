package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/user"
	"path"

	"github.com/gorilla/mux"
	"github.com/mikkeloscar/sshconfig"
)

var hostMap map[string]*sshconfig.SSHHost

var manager = TunnelManager{
	cmd:  make(chan *cmd),
	resp: make(chan bool),
}

func main() {
	var err error
	hostMap, err = getHosts()
	if err != nil {
		panic(err)
	}

	port := flag.Int("p", 4444, "HTTP server port")
	tunnelPort := flag.Int("tp", 5555, "SSH tunnel port")
	flag.Parse()

	go manager.run(*tunnelPort)

	webServer(*port)
}

func getHosts() (map[string]*sshconfig.SSHHost, error) {
	currUser, err := user.Current()
	if err != nil {
		return nil, err
	}

	confPath := path.Join(currUser.HomeDir, ".ssh/config")

	hosts, err := sshconfig.ParseSSHConfig(confPath)
	if err != nil {
		return nil, err
	}

	hostMap := make(map[string]*sshconfig.SSHHost)

	// add possible missing user entries
	for _, host := range hosts {
		if host.User == "" {
			host.User = currUser.Username
		}

		hostMap[host.Host[0]] = host
	}

	return hostMap, nil
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	hostName := r.FormValue("host")

	status := map[string]string{
		"status": "ok",
	}

	if host, ok := hostMap[hostName]; ok {
		// send start command with host to tunnel spawner
		if !manager.Start(host) {
			status["status"] = "error"
		}
		writeJson(w, status)
	} else {
		status["status"] = "error"
		status["msg"] = fmt.Sprintf("invalid host '%s'", hostName)
		writeJson(w, status)
	}
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	// stop current tunnel (if any)
	status := map[string]string{
		"status": "ok",
	}

	if !manager.Stop() {
		status["status"] = "error"
	}
	writeJson(w, status)
}

func writeJson(w http.ResponseWriter, data interface{}) {
	jData, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatalln(err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	jData = append(jData, '\n')
	w.Write(jData)
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	hosts := make([]*sshconfig.SSHHost, 0, len(hostMap))
	for _, host := range hostMap {
		hosts = append(hosts, host)
	}

	writeJson(w, hosts)
}

func webServer(port int) {
	r := mux.NewRouter()
	r.HandleFunc("/start", startHandler).Methods("POST")
	r.HandleFunc("/stop", stopHandler).Methods("POST")
	r.HandleFunc("/info", infoHandler).Methods("GET")

	http.Handle("/", r)

	log.Printf("Serving webserver on http://localhost:%d\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
