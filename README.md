# concourse-summary-gl

Provides a concourse summary as a desktop app

## Build and run

```
go build -ldflags="-s -w" -o disp main.go && ./disp https://buildpacks.ci.cf-app.com
```

or if "ci" is a target in your flyrc file

```
go run main.go ci
```

## With Thanks

Rubik font downloaded from https://fontlibrary.org/en/font/rubik
