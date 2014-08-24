// Package main in mcpulimit is a multi-process cpu-limiting program
//
// Copyright 2013 The Agostle Authors. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.
package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	fExe     = flag.String("e", "", "the name of the executable to watch and limit")
	fLimit   = flag.Int("l", 50, "the percent (between 1 and 100) to limit the processes CPU usage to")
	fTimeout = flag.Int("t", 0, "timeout (seconds) to exit after if there is no suitable target process (lazy mode)")
	fPid     = flag.Int("p", 0, "pid of the process")
)

func main() {
	flag.Parse()
	targets := make([]string, 0, 1)
	if *fExe != "" {
		targets = append(targets, *fExe)
	}
	var err error
	oneSecond := time.Duration(1) * time.Second
	mtx := sync.Mutex{}
	running := true
	procMap := make(map[int]*os.Process, 16)
	if *fPid > 0 {
		var err error
		if procMap[*fPid], err = os.FindProcess(*fPid); err != nil {
			log.Fatalf("cannot find %d: %v", *fPid, err)
		}
	} else {

		if flag.NArg() > 0 {
			targets = append(targets, flag.Args()...)
		}
		for i, exe := range targets {
			if exe[0] != '/' {
				exe, err = exec.LookPath(exe)
				if err != nil {
					log.Printf("cannot find full path for %q: %s", exe, err)
					continue
				}
				targets[i] = exe
			}
		}
		go func() {
			var (
				ok        bool
				null      struct{}
				processes []*os.Process
				oldpids   = make(map[int]struct{}, 16)
				times     int
			)
			for {
				processes = getProcesses(processes[:0], targets)
				if len(processes) == 0 {
					if *fTimeout > 0 {
						times++
						if times > *fTimeout {
							log.Println("no more processes to watch, timeout reached - exiting.")
							running = false
							return
						}
					}
				} else {
					mtx.Lock()
					for k := range procMap {
						oldpids[k] = null
					}
					for _, p := range processes {
						if _, ok = procMap[p.Pid]; !ok {
							log.Printf("new process %d", p.Pid)
						}
						procMap[p.Pid] = p
						delete(oldpids, p.Pid)
					}
					for k := range oldpids {
						log.Printf("%d exited", k)
						delete(procMap, k)
						delete(oldpids, k)
					}
					mtx.Unlock()
				}
				time.Sleep(oneSecond)
			}
		}()
	}

	stopped := false
	var (
		sig   os.Signal
		sleep time.Duration
		n     int64
	)
	tbd := make([]int, 0, 2)
	run := time.Duration(10*(*fLimit)) * time.Millisecond
	freeze := time.Duration(1000)*time.Millisecond - run
	for running {
		mtx.Lock()
		n = int64(len(procMap))
		if n == 0 {
			sleep = oneSecond
		} else {
			if stopped {
				sig, stopped, sleep = syscall.SIGCONT, false, time.Duration(int64(run)/n)
			} else {
				sig, stopped, sleep = syscall.SIGSTOP, true, freeze
			}
			tbd = tbd[:0]
			for pid, p := range procMap {
				if err = p.Signal(sig); err != nil {
					if strings.HasSuffix(err.Error(), "no such process") {
						log.Printf("%d vanished.", pid)
					} else {
						log.Printf("error signaling %d: %s", pid, err)
					}
					tbd = append(tbd, pid)
				}
			}
			if len(tbd) > 0 {
				for _, pid := range tbd {
					delete(procMap, pid)
				}
			}
		}
		mtx.Unlock()
		time.Sleep(sleep)
	}
}

func getProcesses(processes []*os.Process, targets []string) []*os.Process {
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
	if processes == nil {
		processes = make([]*os.Process, 0, len(fis))
	}
	var ok bool
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
		if len(targets) == 0 {
			ok = true
		} else {
			if dst, err = os.Readlink("/proc/" + fi.Name() + "/exe"); err != nil {
				continue
			}
			for _, exe := range targets {
				if exe == dst {
					ok = true
					break
					//log.Printf("dst=%q =?= exe=%q", dst, exe)
				}
			}
		}
		if !ok {
			continue
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
