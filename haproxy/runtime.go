package haproxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/magneticio/vamp-loadbalancer/parsers"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	//"os/signal"
	"strconv"
	"strings"
	//"syscall"
)

func (r *Runtime) SetPid(pidfile string) bool {

	//Create and empty pid file on the specified location, if not already there
	if _, err := os.Stat(pidfile); err == nil {
		return false
	} else {
		emptyPid := []byte("")
		ioutil.WriteFile(pidfile, emptyPid, 0644)
		return true
	}
}

// Reload runtime with configuration
func (r *Runtime) Reload(c *Config) error {

	// Fix for zombie processes kindly provided by https://github.com/QubitProducts/bamboo/issues/31
	// Wait for died children to avoid zombies
	// signalChannel := make(chan os.Signal, 2)
	// signal.Notify(signalChannel, syscall.SIGCHLD)

	// go func() {
	// 	sig := <-signalChannel
	// 	if sig == syscall.SIGCHLD {
	// 		r := syscall.Rusage{}
	// 		for {
	// 			pid, err := syscall.Wait4(-1, nil, 0, &r)
	// 			pidstring := strconv.Itoa(pid)
	// 			if err != nil {
	// 				// fmt.Println("The following pid was already dead... " + pidstring)
	// 			} else {
	// 				fmt.Println("Successfully waited for pid " + pidstring + " to die!")
	// 			}
	// 		}
	// 	}
	// }()

	pid, err := ioutil.ReadFile(c.PidFile)
	if err != nil {
		return err
	}

	/*  Setup all the command line parameters so we get an executable similar to
	    /usr/local/bin/haproxy -f resources/haproxy_new.cfg -p resources/haproxy-private.pid -sf 1234

	*/
	arg0 := "-f"
	arg1 := c.ConfigFile
	arg2 := "-p"
	arg3 := c.PidFile
	arg4 := "-D"
	arg5 := "-sf"
	arg6 := strings.Trim(string(pid), "\n")
	var cmd *exec.Cmd

	//fmt.Println(r.Binary + " " + arg0 + " " + arg1 + " " + arg2 + " " + arg3 + " " + arg4 + " " + arg5 + " " + arg6)
	// If this is the first run, the PID value will be empty, otherwise it will be > 0
	if len(arg6) > 0 {
		cmd = exec.Command(r.Binary, arg0, arg1, arg2, arg3, arg4, arg5, arg6)
	} else {
		cmd = exec.Command(r.Binary, arg0, arg1, arg2, arg3, arg4)
	}

	var out bytes.Buffer
	cmd.Stdout = &out

	cmdErr := cmd.Run()
	if cmdErr != nil {
		return cmdErr
	}

	return nil
}

// Sets the weight of a backend
func (r *Runtime) SetWeight(backend string, server string, weight int) (string, error) {

	result, err := r.cmd("set weight " + backend + "/" + server + " " + strconv.Itoa(weight) + "\n")

	if err != nil {
		return "", err
	} else {
		return result, nil
	}

}

// Adds an ACL.
// We need to match a frontend name to an id. This is somewhat awkard.
func (r *Runtime) SetAcl(frontend string, acl string, pattern string) (string, error) {

	result, err := r.cmd("add acl " + acl + pattern)

	if err != nil {
		return "", err
	} else {
		return result, nil
	}
}

// Gets basic info on haproxy process
func (r *Runtime) GetInfo() (Info, error) {
	var Info Info
	result, err := r.cmd("show info \n")
	if err != nil {
		return Info, err
	} else {
		result, err := parsers.MultiLineToJson(result)
		if err != nil {
			return Info, err
		} else {
			err := json.Unmarshal([]byte(result), &Info)
			if err != nil {
				return Info, err
			} else {
				return Info, nil
			}
		}
	}

}

/* get the basic stats in CSV format

@parameter statsType takes the form of:
- all
- frontend
- backend
*/
func (r *Runtime) GetStats(statsType string) ([]StatsGroup, error) {

	var Stats []StatsGroup
	var cmdString string

	switch statsType {
	case "all":
		cmdString = "show stat -1\n"
	case "backend":
		cmdString = "show stat -1 2 -1\n"
	case "frontend":
		cmdString = "show stat -1 1 -1\n"
	case "server":
		cmdString = "show stat -1 4 -1\n"
	}

	result, err := r.cmd(cmdString)
	if err != nil {
		return Stats, err
	} else {
		result, err := parsers.CsvToJson(strings.Trim(result, "# "))
		if err != nil {
			return Stats, err
		} else {
			err := json.Unmarshal([]byte(result), &Stats)
			if err != nil {
				return Stats, err
			} else {
				return Stats, nil
			}
		}

	}
}

// Executes a arbitrary HAproxy command on the unix socket
func (r *Runtime) cmd(cmd string) (string, error) {

	// connect to haproxy
	conn, err_conn := net.Dial("unix", "/tmp/haproxy.stats.sock")
	defer conn.Close()

	if err_conn != nil {
		return "", errors.New("Unable to connect to Haproxy socket")
	} else {

		fmt.Fprint(conn, cmd)

		response := ""

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			response += (scanner.Text() + "\n")
		}
		if err := scanner.Err(); err != nil {
			return "", err
		} else {
			return response, nil
		}

	}
}