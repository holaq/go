package shell

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const (
	CMD_Flag_Start = 1
	CMD_Flag_Out   = 2
	CMD_Flag_Err   = 3
	CMD_Flag_Over  = 4
)

func ExecCommand(handler func(string, int), commandName string, params ...string) (resStr, errStr string, err error) {
	if handler == nil {
		handler = func(msg string, flag int) {
			fmt.Println("Flag:", flag, ", msg:", msg)
		}
	}
	cmd := exec.Command(commandName, params...)
	handler(fmt.Sprintf("%v", cmd.Args), CMD_Flag_Start)
	var resOS, errOS io.ReadCloser
	if resOS, err = cmd.StdoutPipe(); err == nil {
		defer resOS.Close()
		if errOS, err = cmd.StderrPipe(); err == nil {
			defer errOS.Close()
			if err = cmd.Start(); err == nil {
				resReader := bufio.NewReader(resOS)
				errReader := bufio.NewReader(errOS)
				// isOutAlive, isErrAlive := true, true
				resAppender, errAppender, closer := make(chan string, 10), make(chan string, 10), make(chan int, 2)

				f1 := func() {
					var line string
					for {
						if line, err = resReader.ReadString('\n'); err == nil {
							handler(line, CMD_Flag_Out)
							resStr += line
						} else {
							if err == io.EOF {
								err = nil
							} else {
								handler(err.Error(), CMD_Flag_Err)
							}
							break
						}
					}

					closer <- 1
				}

				f2 := func() {
					var line string
					for {
						if line, err = errReader.ReadString('\n'); err == nil {
							handler(line, CMD_Flag_Err)
							errStr += line
						} else {
							if err == io.EOF {
								err = nil
							} else {
								handler(err.Error(), CMD_Flag_Err)
							}
							break
						}
					}
					closer <- 1
				}

				closeCount := 2

				go f1()
				go f2()

				var s string
			_FOR_LISTEN_:
				for {
					select {
					case <-closer:
						closeCount--
						if closeCount == 0 {
							break _FOR_LISTEN_
						}
					case s = <-resAppender:
						resStr += s
					case s = <-errAppender:
						errStr += s
					}
				}

				// for {
				// 	if isOutAlive {
				// 		if line, err = resReader.ReadString('\n'); err == nil {
				// 			handler(line, CMD_Flag_Out)
				// 			resStr += line
				// 		} else {
				// 			isOutAlive = false
				// 			if err == io.EOF {
				// 				err = nil
				// 			} else {
				// 				handler(err.Error(), CMD_Flag_Err)
				// 			}
				// 		}
				// 	}
				// 	if isErrAlive {
				// 		if line, err = errReader.ReadString('\n'); err == nil {
				// 			handler(line, CMD_Flag_Err)
				// 			errStr += line
				// 		} else {
				// 			isErrAlive = false
				// 			if err == io.EOF {
				// 				err = nil
				// 			} else {
				// 				handler(err.Error(), CMD_Flag_Err)
				// 			}
				// 		}
				// 	}
				// 	if !isOutAlive && !isErrAlive {
				// 		break
				// 	}
				// }
				cmd.Wait()
			}
		}
	}

	if err != nil {
		handler(err.Error(), CMD_Flag_Err)
	}
	handler(resStr, CMD_Flag_Over)

	return
}

func KillProcess(processName string) (killedProgress []string) {
	if resS, _, err := ExecCommand(nil, "pgrep", "-f", processName); err == nil {
		allProcess := strings.Split(resS, "\n")
		killedProgress = []string{}

		for _, pro := range allProcess {
			if len(pro) > 0 {
				if _, _, err = ExecCommand(nil, "kill", pro); err == nil {
					killedProgress = append(killedProgress, pro)
				}
			}
		}
	} else {
		fmt.Println("Ps Command fail:", err)
	}
	return killedProgress
}

func ExecCommandWithPipeAsync(cmds []string, out, err io.Writer, in io.Reader) (cmd *exec.Cmd, e error) {
	switch len(cmds) {
	case 0:
		e = errors.New("Too less command params")
		return
	case 1:
		cmd = exec.Command(cmds[0])
	default:
		cmd = exec.Command(cmds[0], cmds[1:]...)
	}
	if out != nil {
		cmd.Stdout = out
	}
	if err != nil {
		cmd.Stderr = err
	}
	if in != nil {
		cmd.Stdin = in
	}
	e = cmd.Start()
	return
}

func ExecCommandWithPipe(cmds []string, out, err io.Writer, in io.Reader) (e error) {
	var cmd *exec.Cmd
	if cmd, e = ExecCommandWithPipeAsync(cmds, out, err, in); e == nil {
		cmd.Wait()
	}
	return
}

func Exec(cmds ...string) (outMsg, errMsg []byte, e error) {
	var cmd *exec.Cmd
	out, err := bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{})
	if cmd, e = ExecCommandWithPipeAsync(cmds, out, err, nil); e == nil {
		cmd.Wait()
		outMsg, errMsg = out.Bytes(), err.Bytes()
	}
	return
}
