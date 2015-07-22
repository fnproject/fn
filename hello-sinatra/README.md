

```
docker run --rm -p 8080:8080 treeder/hello-sinatra
```



## Building

Update gems and vendor them
```
dj run treeder/ruby:2.2.2 bundle update
dj run treeder/ruby:2.2.2 bundle install --standalone --clean
```

```
docker build -t treeder/hello-sinatra:latest .
```
