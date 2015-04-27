package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"os/user"
	"path"

	"github.com/gorilla/mux"
	"github.com/mikkeloscar/sshconfig"
)

var hostMap map[string]*sshconfig.SSHHost
var manager *tunnelManager

type cmd struct {
	Cmd  string
	Host *sshconfig.SSHHost
}

type tunnelManager struct {
	process *exec.Cmd
	cmd     chan *cmd
	resp    chan bool
}

func (t *tunnelManager) run() {
	for {
		cmd := <-t.cmd
		switch cmd.Cmd {
		case "start":
			fmt.Printf("starting: %s\n", cmd.Host.HostName)
			err := t.start(cmd.Host)
			if err != nil {
				t.resp <- false
			} else {
				t.resp <- true
			}
		case "stop":
			err := t.stop()
			if err != nil {
				t.resp <- false
			} else {
				t.resp <- true
			}
		}
	}
}

func (t *tunnelManager) start(host *sshconfig.SSHHost) error {
	// stop other process if running
	t.stop()

	t.process = exec.Command("ssh", "-D", "1080", "-N",
		fmt.Sprintf("%s@%s", host.User, host.HostName),
		"-p", fmt.Sprintf("%d", host.Port))

	return t.process.Start()
}

func (t *tunnelManager) stop() error {
	if t.process != nil {
		return t.process.Process.Kill()
	}
	return nil
}

func (t *tunnelManager) Start(host *sshconfig.SSHHost) bool {
	t.cmd <- &cmd{"start", host}
	return <-t.resp
}

func (t *tunnelManager) Stop() bool {
	t.cmd <- &cmd{"stop", nil}
	return <-t.resp
}

func main() {
	var err error
	hostMap, err = getHosts()
	if err != nil {
		panic(err)
	}

	manager = &tunnelManager{
		cmd:  make(chan *cmd),
		resp: make(chan bool),
	}

	// start tunnel Manager
	go manager.run()

	port := flag.Int("p", 4444, "server port")
	flag.Parse()

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

	if host, ok := hostMap[hostName]; ok {
		// send start command with host to tunnelManager
		if manager.Start(host) {
			fmt.Fprintf(w, "ok\n")
		} else {
			fmt.Fprintf(w, "error\n")
		}
	} else {
		fmt.Fprintf(w, "error\n")
	}
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	// stop current active tunnnel (if any)
	if manager.Stop() {
		fmt.Fprintf(w, "ok\n")
	} else {
		fmt.Fprintf(w, "error\n")
	}
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	hosts := make([]*sshconfig.SSHHost, 0, len(hostMap))
	for _, host := range hostMap {
		hosts = append(hosts, host)
	}

	jHosts, err := json.Marshal(hosts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatalln(err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jHosts)
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
