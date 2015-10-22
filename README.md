This is WIP of a go cli port to the odoo community.


Here is an open design proposal on what to be implemented, if you have any ideas, please refer to [CONTRIBUTING.md](ONTRIBUTING.md)

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