# IronFunctions using Postgres

Let's presuppose you don't have even a MySQL DB ready.

### 1. Let's start a MySQL instance:

```
docker run --name iron-mysql \
        -e MYSQL_DATABASE=funcs -e MYSQL_USER=iron -e MYSQL_PASSWORD=ironfunctions -d mysql
``` 

For more configuration options, see [docker mysql docs](https://hub.docker.com/_/mysql/).

### 2. Now let's start IronFunctions connecting to our new mysql instance

```
docker run --rm --privileged --link "iron-mysql:mysql" \
    -e "DB_URL=mysql://iron:ironfunctions@tcp(mysql:3306)/funcs" \
    -it -p 8080:8080 iron/functions
```
