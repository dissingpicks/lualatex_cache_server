package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type logDestination struct {
	mutex  sync.Mutex
	buffer []byte
	pipe   io.Writer
}

func (d *logDestination) Write(b []byte) (int, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if d.pipe != nil {
		return d.pipe.Write(b)
	} else {
		d.buffer = append(d.buffer, b...)
		return len(b), nil
	}
}

func (d *logDestination) SetPipe(p io.Writer) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.pipe = p
	if len(d.buffer) != 0 {
		p.Write(d.buffer)
	}
}

func newLogDestination() *logDestination {
	return &logDestination{buffer: make([]byte, 0)}
}

type LatexEngine struct {
	Path         string
	cmd          *exec.Cmd
	currentDir   string
	currentFile  string
	currentArg   []string
	preamble     []byte
	cmdStdinPipe io.WriteCloser
	logDst       *logDestination
}

func (e *LatexEngine) parseSrc() ([]byte, []byte, error) {
	fullSrcPath := filepath.Join(e.currentDir, e.currentFile)
	src, err := os.ReadFile(fullSrcPath)
	if err != nil {
		return nil, nil, err
	}
	beginDocInd := bytes.Index(src, []byte("\n\\begin{document}"))
	if beginDocInd == -1 {
		return nil, nil, errors.New("could not find \\begin{document}")
	}
	return src[:beginDocInd], src[beginDocInd+1:], nil
}

func (e *LatexEngine) cachePreamble(invalidate bool) {
	e.logDst = newLogDestination()
	e.cmd = e.makeCommand(e.logDst)
	pipe, err := e.cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	e.cmdStdinPipe = pipe
	go func() {
		if invalidate {
			preamble, _, err := e.parseSrc()
			if err != nil {
				return
			}
			e.preamble = preamble
		}

		if err := e.cmd.Start(); err != nil {
			panic(err)
		}
		_, err = pipe.Write(e.preamble)
		if err != nil {
			panic(err)
		}
	}()
}

func (e *LatexEngine) jobname() string {
	return strings.TrimSuffix(e.currentFile, filepath.Ext(e.currentFile))
}

func (e *LatexEngine) makeCommand(logDst io.Writer) *exec.Cmd {
	cmd := exec.Command(e.Path, append(e.currentArg, "--jobname="+e.jobname())...)
	cmd.Dir = e.currentDir
	cmd.Stdout = logDst
	cmd.Stderr = logDst
	return cmd
}

func (e *LatexEngine) handleCacheMiss(logDst io.Writer) int {
	if e.cmd != nil {
		if err := e.cmdStdinPipe.Close(); err != nil {
			panic(err)
		}
		if err := e.cmd.Wait(); err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				panic(err)
			}
		}
	}

	cmd := e.makeCommand(logDst)
	cmd.Args = append(cmd.Args, e.currentFile)
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			logDst.Write([]byte(err.Error()))
			return -1
		}
	}

	e.cachePreamble(true)
	return cmd.ProcessState.ExitCode()
}

func (e *LatexEngine) Typeset(logDst io.Writer, dir string, args []string, file string) int {
	if e.cmd == nil || e.currentDir != dir || e.currentFile != file || !slices.Equal(e.currentArg, args) {
		e.currentDir = dir
		e.currentFile = file
		e.currentArg = args
		return e.handleCacheMiss(logDst)
	}
	preamble, mainContent, err := e.parseSrc()
	if err != nil {
		logDst.Write([]byte(err.Error()))
		return -1
	}
	if !bytes.Equal(preamble, e.preamble) {
		return e.handleCacheMiss(logDst)
	}
	e.logDst.SetPipe(logDst)
	e.cmdStdinPipe.Write(mainContent)
	e.cmdStdinPipe.Close()
	e.cmd.Wait()
	exitCode := e.cmd.ProcessState.ExitCode()
	e.cachePreamble(false)
	return exitCode
}

func (e *LatexEngine) CurrentCache() string {
	return filepath.Join(e.currentDir, e.currentFile)
}

func (e *LatexEngine) Terminate() {
	if e.cmd != nil {
		e.cmdStdinPipe.Close()
		e.cmd.Wait()
	}
}
