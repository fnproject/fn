# IronFunctions using Postgres

Let's presuppose you don't have even a postgres DB ready.

### 1. Let's start a postgres instance:

```
docker run --name iron-postgres \
        -e POSTGRES_PASSWORD=ironfunctions -d postgres
``` 

### 2. Now let's create a new database to IronFunctions

Creating database:

```
docker run -it --rm --link iron-postgres:postgres postgres \
    psql -h postgres -U postgres -c "CREATE DATABASE funcs;"
```

Granting access to postgres user

```
docker run -it --rm --link iron-postgres:postgres postgres \
    psql -h postgres -U postgres -c 'GRANT ALL PRIVILEGES ON DATABASE funcs TO postgres;'
```

### 3. Now let's start IronFunctions connecting to our new postgres instance

```
docker run --rm --privileged --link "iron-postgres:postgres" \
    -e "DB_URL=postgres://postgres:ironfunctions@postgres/funcs?sslmode=disable" \
    -it -p 8080:8080 iron/functions
```
