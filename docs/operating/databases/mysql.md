# Fn using MySQL DB

Let's presuppose you don't have even a MySQL DB ready.

### 1. Let's start a MySQL instance:

```
docker run --name func-mysql \
        -e MYSQL_DATABASE=funcs -e MYSQL_USER=func -e MYSQL_PASSWORD=funcpass -e MYSQL_RANDOM_ROOT_PASSWORD=yes -d mysql
```

For more configuration options, see [docker mysql docs](https://hub.docker.com/_/mysql/).

### 2. Now let's start Functions connecting to our new mysql instance

```
docker run --rm --privileged --link "func-mysql:mysql" \
    -e "DB_URL=mysql://func:funcpass@tcp(mysql:3306)/funcs" \
    -it -p 8080:8080 fnproject/fn
```
