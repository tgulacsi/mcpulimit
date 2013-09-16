# mcpulimit #
[cpulimit](https://github.com/opsengine/cpulimit)-like program for more than
process.

## Use case ##
I have a program with starts several child processes (GraphicsMagic),
which each starts GhostScript subprocess, which eats cpu and together
overheats my tiny laptop, which reboots.

## Usage ##
    mcpulimit [OPTIONS] [executable names]
        -l=50: the percent (between 1 and 100) to limit the processes CPU usage to
        -t=0: timeout (seconds) to exit after if there is no suitable target process (lazy mode)

The executable must be full paths if the executable is not found in PATH.
mcpulimit sends `SIGSTOP` and `SIGCONT` signals to the processes, thus you
need to have permission for this - but practically if mcpulimit runs as the
same user as the processes to be limited, no problems araise.

## Installation ##
    go install github.com/tgulacsi/mcpulimit

### Requirements ###
Go compiler and an operating system with `/proc` filesystem, and `SIGSTOP` and `SIGCONT` signals.
