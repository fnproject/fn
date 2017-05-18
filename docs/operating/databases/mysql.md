# Oracle Functions using Postgres

Let's presuppose you don't have even a MySQL DB ready.

### 1. Let's start a MySQL instance:

```
docker run --name func-mysql \
        -e MYSQL_DATABASE=funcs -e MYSQL_USER=funcy -e MYSQL_PASSWORD=funcypass -d mysql
``` 

For more configuration options, see [docker mysql docs](https://hub.docker.com/_/mysql/).

### 2. Now let's start Functions connecting to our new mysql instance

```
docker run --rm --privileged --link "iron-mysql:mysql" \
    -e "DB_URL=mysql://funcy:funcypass@tcp(mysql:3306)/funcs" \
    -it -p 8080:8080 treeder/functions
```
