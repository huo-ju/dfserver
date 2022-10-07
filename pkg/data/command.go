package data

import (
	"strings"
)

type CommandArgs struct {
	Cmd  string
	Args []string
}

type Pstatus int

const (
	StatusCmd Pstatus = iota
	StatusArgPrefix
	StatusArgSkip
	StatusArgItemKStart
	StatusArgItemVStart
	StatusArgItemEnd
)

func ArgsParse(input string, argslist []string) *CommandArgs {
	s := StatusCmd
	indexargstart := len(input)
	buffer := ""

End:
	for i, c := range input {
		switch s {
		case StatusCmd:
			if c == '-' {
				s = StatusArgPrefix
				indexargstart = i
			}
		case StatusArgPrefix:
			if c == ' ' {
				//if args in the list?
				buffer = input[indexargstart:i]

				ifarg := false
				for _, a := range argslist {
					if buffer == a {
						ifarg = true
						break
					}
				}
				if ifarg == true {
					indexargstart = indexargstart - 1
					break End
				} else {
					s = StatusCmd
					indexargstart = len(input)
				}
			} else if c == '|' {
				if i-indexargstart == 2 { // xxxx:-1|, is not a args
					indexargstart = len(input) //recovery the init value
					s = StatusCmd
				}
			}
		}
	}

	argstr := input[indexargstart:]
	args := []string{}
	if len(argstr) > 0 {
		s = StatusArgSkip
		startidx := 0
		endidx := len(argstr)
		for i, c := range argstr {
			switch s {
			case StatusArgSkip:
				if c == '-' {
					s = StatusArgItemKStart
					startidx = i
				}
			case StatusArgItemKStart:
				if c == ' ' {
					s = StatusArgItemVStart
				}
				endidx = i
			case StatusArgItemVStart:
				if c == '-' {
					s = StatusArgItemEnd
				} else {
					endidx = i
				}

			case StatusArgItemEnd:
				args = append(args, strings.TrimSpace(argstr[startidx:endidx]))
				s = StatusArgItemKStart
				//reset index
				startidx = i - 1
				endidx = startidx
			}
		}
		if endidx < len(argstr) {
			args = append(args, strings.TrimSpace(argstr[startidx:]))
		}
	}

	cmdargs := &CommandArgs{Cmd: input[:indexargstart], Args: args}
	return cmdargs
}
