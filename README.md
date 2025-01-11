# LuaLaTeX cache server
This tool acts as a transparent wrapper of LuaLaTeX, but tries to make the typesetting fast by caching the preamble part.

# Requirements
`lualatex` executable located at somewhere in the `$PATH` environment variable.

# Install
Precompiled binaries are available at the release page (only tested on macOS).

https://github.com/dissingpicks/lualatex_cache_server/releases

One can also build from source.
```
go install github.com/dissingpicks/lualatex_cache_server@latest
```

# Usage
`lualatex_cache_server` can be used as if it were `lualatex` except for `--interaction` and `--jobname`.
These options are not compatible with `lualatex_cache_server`, and one must not specify them.

For example, one may use `lualatex_cache_server` like this:
```
lualatex_cache_server --synctex=1 --file-line-error main.tex
```

`lualatex_cache_server` accepts the following its own options.
These options must be specified at the beginning of the command line arguments.
- `--port`: Specify the port number which server listens on (default: 59603).
- `--nobrowser`: Prevent it from opening the browser to show the status page.
- `--launch`: Launch the server. One usually do not have to use this option.

This example illustrates how the above options are used:
```
lualatex_cache_server --port=8080 --nobrowser --synctex=1 --file-line-error main.tex
```

# How it works
This tool launches `lualatex` in background and feed the preamble part through STDIN.

Then, it feeds from `\begin{document}` to the end of file when requested.
