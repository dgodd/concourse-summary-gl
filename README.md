# concourse-summary-gl

Provides a concourse summary as a desktop app

## Build and run

```
go build -ldflags="-s -w" -o disp http.go main.go getdata.go && ./disp buildpacks.ci.cf-app.com
```

## With Thanks

Rubik font downloaded from https://fontlibrary.org/en/font/rubik
