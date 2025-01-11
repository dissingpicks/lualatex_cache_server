package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/browser"
)

type launchMsgReceiver struct {
	msg      []byte
	received chan bool
}

func (r *launchMsgReceiver) Write(b []byte) (int, error) {
	r.msg = append(r.msg, b...)
	if bytes.HasPrefix(r.msg, []byte(SuccessfulLaunchMsg)) {
		r.received <- true
	}
	return len(b), nil
}

func launchServer() {
	executablePath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	launchSucceededCh := make(chan bool)
	launchChecker := &launchMsgReceiver{
		msg:      make([]byte, 0, len(SuccessfulLaunchMsg)),
		received: launchSucceededCh,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executablePath, append([]string{"--launch"}, os.Args[1:]...)...)
	cmd.Stdout = launchChecker
	cmd.Stderr = os.Stderr
	cmd.Cancel = func() error {
		launchSucceededCh <- false
		return nil
	}
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	go func() {
		cmd.Wait()
		launchSucceededCh <- false
	}()
	if !(<-launchSucceededCh) {
		cmd.Process.Kill()
		panic("failed to launch a server")
	}
	cmd.Process.Release()
}

func client(rawURL string) error {
	response, err := http.Get(rawURL)
	if err != nil {
		return err
	}
	io.Copy(os.Stdout, response.Body)
	trailer := response.Trailer
	if trailer != nil {
		code := trailer.Get(StatusCodeTrailerKey)
		if code != "" {
			n, err := strconv.ParseInt(code, 10, 64)
			if err == nil {
				os.Exit(int(n))
			}
		}
	}
	panic("failed to receive the exit code")
}

func main() {
	arguments := os.Args[1:]
	port := uint(59603)
	launch := false
	showBrowser := true
	for i, arg := range arguments {
		const portFlag = "--port="
		const launchFlag = "--launch"
		const noBrowserFlag = "--nobrowser"
		if strings.HasPrefix(arg, portFlag) {
			p, err := strconv.ParseUint(arg[len(portFlag):], 10, 64)
			if err != nil {
				panic(err)
			}
			port = uint(p)
		} else if arg == launchFlag {
			launch = true
		} else if arg == noBrowserFlag {
			showBrowser = false
		} else {
			arguments = arguments[i:]
			break
		}
	}

	if launch {
		new(Server).Run(port)
	}

	apiURL, err := url.Parse(fmt.Sprintf("http://localhost:%v/"+TypesetAPIPath, port))
	if err != nil {
		panic(err)
	}

	query := apiURL.Query()
	for _, arg := range arguments {
		query.Add(ArgumentsQueryKey, arg)
	}

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	query.Set(DirectoryQueryKey, cwd)
	apiURL.RawQuery = query.Encode()

	rawURL := apiURL.String()
	if client(rawURL) != nil {
		launchServer()
		if showBrowser {
			browser.OpenURL(fmt.Sprintf("http://localhost:%v", port))
		}
		if err := client(rawURL); err != nil {
			panic(err)
		}
	}
}
