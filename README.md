This is WIP of a go cli port to the odoo community.


Here is an open design proposal on what to be implemented, if you have any ideas, please refer to [CONTRIBUTING.md](CONTRIBUTING.md)

vodoo command [arguments]
```
YES    build       compile packages and dependencies -> Prepare (travis2docker) and Build a Dockerfile / docker-compose.yaml
NO     clean       remove object files
YES    doc         run godoc on package sources      -> Some integration possible with rtfd.org, like prepare some folders, defaults?
YES    env         print Go environment information  -> Just very basic like $VODOOPATH
NO     fix         run go tool fix on packages
YES    fmt         run gofmt on package sources      -> Probably integration with YAPF
YES    get         download and install packages and dependencies -> integrate anybox functionality
NO     install     compile and install packages and dependencies
YES    list        list packages                     -> See what packagase/modules are on the $VODOOPATH
YES    run         compile and run Go program		 -> Wrapper around docker-compose or similar
YES    test        test packages					 -> like run with test flag set.
NO     tool        run specified go tool
YES    version     print Go version    				 -> Just a basic version printout.
NO     vet         run go tool vet on packages
```

run/test would rely on build, which would automatically build a docker image and run it.


As for the get command, there are currently identified two sructs, which we can hijack for the odoo use case:

1. The *Package struct, which hold's a package (= "odoo-modules") meta information.
	https://github.com/blaggacao/vodoo/blob/1486b12afc6acdfc8cb6730095b7a4588e8f1895/pkg.go#L23-L78
2. The buildContext which contains information about the language (which could be manipulated in an odoo specific way)
	I'm still not able to figure out, where exactly this is constructed, but it contains:
	```
	build.Context{GOARCH:"386", GOOS:"windows", GOROOT:"B:\\Go\\", GOPATH:"B:\\GoPath", CgoEnabled:true, UseAllFiles:false, Compiler:"gc", BuildTags:[]string(nil), ReleaseTags:[]string{"go1.1", "go1.2", "go1.3", "go1.4"}, InstallSuffix:"", JoinPath:(func(...string) string)(nil), SplitPathList:(func(string) []string)(nil), IsAbsPath:(func(string) bool)(nil), IsDir:(func(string) bool)(nil), HasSubdir:(func(string, string) (string, bool))(nil), ReadDir:(func(string) ([]os.FileInfo, error))(nil), OpenFile:(func(string) (io.ReadCloser, error))(nil)}
	```
Next goal would be to hack the GOPATH and make it an VODOOPATH to have an early demonstrate binary, that would be already functional for setting up and organizing local odoo repositories.
