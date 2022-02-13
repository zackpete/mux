# mux

## Installation

```
go install github.com/zackpete/mux@latest
```

## Help

```
NAME
	mux - a command multiplexer

USAGE
	mux { [options...] <command> } [{ [options...] <command> } ...]

OPTIONS
	name=<string>  prefix each line of output with this name
	exit=<number>  exit with this code when the command exits

EXAMPLES
	mux { echo hello } { echo world }
	mux { name=good ping -c1 example.com } { name=bad ping -c1 example.invalid }
	mux { exit=42 false } { sleep 1 } 
```

## Examples

```
[user@hostname]$ mux { echo hello } { echo world } # order may differ
| hello
| world
```

```
[user@hostname]$ mux { name=good ping -c1 example.com } { name=bad ping -c1 example.invalid }
bad  ! ping: example.invalid: Name or service not known
bad  ? exit status 2
good | PING example.com (93.184.216.34) 56(84) bytes of data.
good | 64 bytes from 93.184.216.34 (93.184.216.34): icmp_seq=1 ttl=56 time=14.1 ms
good |
good | --- example.com ping statistics ---
good | 1 packets transmitted, 1 received, 0% packet loss, time 0ms
good | rtt min/avg/max/mdev = 14.055/14.055/14.055/0.000 ms
```

```
[user@hostname]$ mux { exit=42 false } { sleep 1 }
? exit status 1
[user@hostname]$ echo $?
42
```
