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
)

func main() {
	flag.Parse()
	exe := *fExe
	if exe == "" {
		exe = flag.Arg(0)
	}
	var err error
	if exe[0] != '/' {
		exe, err = exec.LookPath(exe)
		if err != nil {
			log.Fatalf("cannot find full path for %q: %s", exe, err)
		}
	}
	oneSecond := time.Duration(1) * time.Second
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
			time.Sleep(oneSecond)
		}
	}()

	stopped := false
	var (
		sig   os.Signal
		sleep time.Duration
		n     int64
	)
	tbd := make([]int, 0, 2)
	run := time.Duration(10*(*fLimit)) * time.Millisecond
	freeze := time.Duration(1000)*time.Millisecond - run
	for {
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
