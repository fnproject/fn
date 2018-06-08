# Blog API in 10 minutes

This is a simple blog API with a function to receive a list of posts and a function to create a post.

## Run it

```
# Start MySQL:
docker run --name mysql --net=host -p 3306:3306 -e MYSQL_ROOT_PASSWORD=pass -d mysql:8
docker run -it --rm --link mysql:mysql mysql mysql -ppass -hmysql -e "create database blog"
docker run -it --rm --link mysql:mysql mysql mysql -ppass -hmysql -e "show databases"

# create schema
fn run -e DB_USER=root -e DB_PASS=pass schema

# Test locally:
# Check if any posts, should be none
fn run -e DB_USER=root -e DB_PASS=pass posts
# Add one
cat post.json | fn run -e DB_USER=root -e DB_PASS=pass posts/create
# Check again
fn run -e DB_USER=root -e DB_PASS=pass posts

# Set app configs
fn config app blog DB_USER root
fn config app blog DB_PASS pass

# fn deploy it!
fn deploy --all
```

Now surf to: http://localhost/r/blog/

To create posts:

```
echo '{
    "title": "Blog Post 1",
    "body": "This is the body. This is the body. This is the body. This is the body. This is the body. This is the body. "
}' | fn call blog /posts/create
```

To get posts:

```
fn call blog /posts
```

## TODO:

* [ ] Add some way to ignore funcs on deploy, ie: schema
