package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
)

const SuccessfulLaunchMsg = "Launched\n"
const DirectoryQueryKey = "dir"
const ArgumentsQueryKey = "args"
const StatusCodeTrailerKey = "LaTeXStatusCode"
const TypesetAPIPath = "typeset"

type Server struct {
	latex *LatexEngine
}

func findFileNameLike(args []string) int {
	return slices.IndexFunc(args, func(a string) bool {
		return a[0] != '-'
	})
}

func (s *Server) parseArguments(args []string) ([]string, string, bool) {
	srcFileInd := findFileNameLike(args)
	if srcFileInd == -1 {
		return nil, "", false
	}
	if findFileNameLike(args[srcFileInd+1:]) != -1 {
		return nil, "", false
	}
	srcFile := args[srcFileInd]
	return slices.Delete(args, srcFileInd, srcFileInd+1), srcFile, true
}

func (s *Server) typeset(logDst io.Writer, query url.Values) (int, bool) {
	if !query.Has(DirectoryQueryKey) {
		return -1, false
	}
	if !query.Has(ArgumentsQueryKey) {
		return -1, false
	}
	arguments, srcFile, ok := s.parseArguments(query[ArgumentsQueryKey])
	if !ok {
		return -1, false
	}
	statusCode := s.latex.Typeset(logDst, query.Get(DirectoryQueryKey), arguments, srcFile)
	return statusCode, true
}

func (s *Server) typesetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Trailer", StatusCodeTrailerKey)
	query := r.URL.Query()
	statusCode, ok := s.typeset(w, query)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
	}
	w.Header().Set(StatusCodeTrailerKey, fmt.Sprintf("%v", statusCode))
}

func (s *Server) Run(port uint) {
	engine, err := exec.LookPath("lualatex")
	if err != nil {
		panic(err)
	}
	s.latex = &LatexEngine{Path: engine}

	server := &http.Server{
		Addr: fmt.Sprintf(":%v", port),
	}
	gracefullyClosed := make(chan any)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "This is LuaLaTeX build server<br/>")
		fmt.Fprint(w, "Current cached file: "+s.latex.CurrentCache()+"<br/>")
		fmt.Fprint(w, `To quit, click this link -> <a href="/quit">quit</a>`)
	})
	http.HandleFunc("/quit", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "The server is terminated")
		go func() {
			if err := server.Shutdown(context.Background()); err != nil {
				panic(err)
			}
			close(gracefullyClosed)
		}()
	})
	http.HandleFunc("/"+TypesetAPIPath, s.typesetHandler)
	sock, err := net.Listen("tcp", server.Addr)
	if err != nil {
		panic(err)
	}
	fmt.Fprint(os.Stdout, SuccessfulLaunchMsg)
	if err := server.Serve(sock); err != http.ErrServerClosed {
		panic(err)
	}
	<-gracefullyClosed
	s.latex.Terminate()
	os.Exit(0)
}
