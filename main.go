package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type (
	Command struct {
		Config     *Config
		Name       string
		Executable string
		Arguments  []string
		Exit       bool
		Code       int

		output chan Line
	}

	Config struct {
		Names bool
		Width int
	}

	State int
	Type  int

	Line struct {
		Type  Type
		Value string
	}

	BlockingReader struct{}
)

const name = "mux"

const (
	Start State = iota
	Option
	Executable
	Arguments
)

const (
	Aux Type = iota
	Out
	Err
)

var (
	Param  = regexp.MustCompile(`^(\w+)=(.+)$`)
	Escape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

func main() {
	if len(os.Args) < 2 {
		die("expected argument")
	}

	switch os.Args[1] {
	case "-h":
		fallthrough
	case "--help":
		fallthrough
	case "help":
		help()
		return
	}

	var config Config
	var commands []*Command
	current := new(Command)

	state := Start

	for i, arg := range os.Args[1:] {
	parse:
		switch state {
		case Start:
			if arg == "{" {
				state = Option
			} else {
				invalid("expected '{'", i)
			}
		case Option:
			if matches := Param.FindStringSubmatch(arg); len(matches) > 0 {
				key := matches[1]
				val := matches[2]

				switch key {
				case "name":
					current.Name = val
					config.Names = true
					config.Width =
						int(math.Max(float64(config.Width), float64(len(val))))
				case "exit":
					if n, err := strconv.Atoi(val); err != nil {
						invalid("option value should be a number", i)
					} else {
						current.Exit = true
						current.Code = n
					}
				default:
					invalid(fmt.Sprintf("unknown option '%s'", key), i)
				}
			} else {
				state = Executable
				goto parse
			}
		case Executable:
			current.Executable = arg
			state = Arguments
		case Arguments:
			if arg == "}" {
				current.Config = &config
				commands = append(commands, current)
				current = new(Command)
				state = Start
			} else {
				current.Arguments = append(current.Arguments, arg)
			}
		default:
			panic("unreachable")
		}
	}

	var cases []reflect.SelectCase

	for _, c := range commands {
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(c.run()),
		})
	}

	closed := 0

	for closed < len(cases) {
		if i, value, ok := reflect.Select(cases); ok {
			line := value.Interface().(Line)
			switch line.Type {
			case Aux:
				fmt.Fprintln(os.Stderr, line.Value)
			case Out:
				fmt.Fprint(os.Stdout, line.Value)
			case Err:
				fmt.Fprint(os.Stderr, line.Value)
			default:
				panic("unreachable")
			}
		} else if c := commands[i]; c.Exit {
			os.Exit(c.Code)
		} else {
			cases[i] = reflect.SelectCase{Dir: reflect.SelectSend, Chan: reflect.Value{}}
			closed++
		}
	}
}

func (this *Command) run() chan Line {
	this.output = make(chan Line)

	cmd := exec.Command(this.Executable, this.Arguments...)

	outR, outW := io.Pipe()
	errR, errW := io.Pipe()

	cmd.Stdout = outW
	cmd.Stderr = errW
	cmd.Stdin = BlockingReader{}

	if err := cmd.Start(); err != nil {
		go func() {
			this.write(Aux, err.Error())
			close(this.output)
		}()

		return this.output
	}

	wg := new(sync.WaitGroup)
	this.pipe(outR, Out, wg)
	this.pipe(errR, Err, wg)

	go func() {
		defer close(this.output)

		err := cmd.Wait()

		outW.Close()
		errW.Close()

		wg.Wait()

		if err != nil {
			this.write(Aux, err.Error())
		}
	}()

	return this.output
}

func (this *Command) pipe(src *io.PipeReader, kind Type, wg *sync.WaitGroup) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		for r := bufio.NewReader(src); ; {
			if line, err := r.ReadString('\n'); err != nil {
				if err == io.EOF {
					if line != "" {
						this.write(kind, line)
					}
				} else {
					this.write(Aux, err.Error())
				}

				return
			} else {
				this.write(kind, line)
			}
		}
	}()
}

func (this *Command) write(kind Type, line string) {
	line = Escape.ReplaceAllString(line, "")

	l := Line{Type: kind}

	var divider string

	if kind == Aux {
		divider = "! "
	} else {
		divider = "| "
	}

	var name string

	if this.Name == "" {
		if this.Config.Names {
			name = fmt.Sprintf("%s ", strings.Repeat(" ", this.Config.Width+1))
		}
	} else {
		space := this.Config.Width - len(this.Name)
		name = fmt.Sprintf("%s %s", this.Name, strings.Repeat(" ", space))
	}

	l.Value = fmt.Sprintf("%s%s%s", name, divider, line)

	this.output <- l
}

func die(msg string) {
	fmt.Printf("%s: %s\n", name, msg)
	os.Exit(1)
}

func invalid(msg string, arg int) {
	die(fmt.Sprintf("argument %d: %s", arg+1, msg))
}

func help() {
	fmt.Printf(
		`NAME
	%[1]s - a command multiplexer

USAGE
	%[1]s { [options...] <command> } [{ [options...] <command> } ...]

OPTIONS
	name=<string>  prefix each line of output with this name
	exit=<number>  exit with this code when the command exits

EXAMPLES
	%[1]s { echo hello } { echo world }
	%[1]s { name=good ping -c1 example.com } { name=bad ping -c1 example.invalid }
	%[1]s { exit=42 false } { sleep 1 } 
`, name)
}

func (_ BlockingReader) Read(_ []byte) (int, error) {
	select {}
}
