curl -H "Content-Type: application/json" -X POST -d '{
    "app": { "name":"myapp" }
}' http://localhost:8080/v1/apps

curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "type": "sync",
        "path":"/hello-sync",
        "image":"iron/hello"
    }
}' http://localhost:8080/v1/apps/myapp/routes

curl -H "Content-Type: application/json" -X POST -d '{
    "name":"Johnny"
}' http://localhost:8080/r/myapp/hello-sync

