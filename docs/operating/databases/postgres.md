# Fn using Postgres

Let's presuppose you don't have even a postgres DB ready.

### 1. Let's start a postgres instance:

```
docker run --name func-postgres \
        -e POSTGRES_PASSWORD=funcpass -d postgres
```

### 2. Now let's create a new database for Functions

Creating database:

```
docker run -it --rm --link func-postgres:postgres postgres \
    psql -h postgres -U postgres -c "CREATE DATABASE funcs;"
```

Granting access to postgres user

```
docker run -it --rm --link func-postgres:postgres postgres \
    psql -h postgres -U postgres -c 'GRANT ALL PRIVILEGES ON DATABASE funcs TO postgres;'
```

### 3. Now let's start Functions connecting to our new postgres instance

```
docker run --rm --privileged --link "func-postgres:postgres" \
    -e "DB_URL=postgres://postgres:funcpass@postgres/funcs?sslmode=disable" \
    -it -p 8080:8080 fnproject/functions
```
