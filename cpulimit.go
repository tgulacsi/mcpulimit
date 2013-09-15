package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

func main() {
	flag.Parse()
	exe := flag.Arg(0)
	var err error
	if exe[0] != '/' {
		exe, err = exec.LookPath(exe)
		if err != nil {
			log.Fatalf("cannot find full path for %q: %s", exe, err)
		}
	}
	procMap := make(map[int]*os.Process, 16)
	mtx := sync.Mutex{}
	go func() {
		for {
			processes := getProcesses(exe)
			if len(processes) > 0 {
				mtx.Lock()
				for k := range procMap {
					delete(procMap, k)
				}
				for _, p := range processes {
					procMap[p.Pid] = p
				}
				mtx.Unlock()
			}
			time.Sleep(time.Duration(1) * time.Second)
		}
	}()

	stopped := false
	var sig os.Signal
	var sleep time.Duration
    tbd := make([]int, 0, 2)
	run := time.Duration(250) * time.Millisecond
	freeze := time.Duration(750) * time.Millisecond
	for {
		mtx.Lock()
		if stopped {
			sig, stopped, sleep = syscall.SIGCONT, false, run
		} else {
			sig, stopped, sleep = syscall.SIGSTOP, true, freeze
		}
        tbd = tbd[:0]
		for pid, p := range procMap {
			if err = p.Signal(sig); err != nil {
				log.Printf("error signaling %s: %s", pid, err)
                tbd = append(tbd, pid)
			}
		}
        if len(tbd) > 0 {
            for _, pid := range tbd {
                delete(procMap, pid)
            }
        }
		mtx.Unlock()
		time.Sleep(sleep)
	}
}

func getProcesses(exe string) []*os.Process {
	dh, err := os.Open("/proc")
	if err != nil {
		log.Fatalf("cannot open /proc: %s", err)
	}
	defer dh.Close()
	fis, err := dh.Readdir(-1)
	if err != nil {
		log.Fatalf("cannot read /proc: %s", err)
	}
	var dst string
	processes := make([]*os.Process, 0, len(fis))
	for _, fi := range fis {
		if !fi.Mode().IsDir() {
			continue
		}
		if !isAllDigit(fi.Name()) {
			continue
		}
		pid, err := strconv.Atoi(fi.Name())
		if err != nil {
			continue
		}
		if exe != "" {
			if dst, err = os.Readlink("/proc/" + fi.Name() + "/exe"); err != nil {
				continue
			}
			//log.Printf("dst=%q =?= exe=%q", dst, exe)
			if dst != exe {
				continue
			}
		}
		p, err := os.FindProcess(pid)
		if err != nil {
			log.Printf("cannot find process %d: %s", pid, err)
		}
		processes = append(processes, p)
	}
	return processes
}

func isAllDigit(name string) bool {
	for _, c := range name {
		if c < '0' || c >= '9' {
			return false
		}
	}
	return true
}
