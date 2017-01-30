# Postgres INSERT/SELECT Function Image

This function executes an INSERT or SELECT against a table in a given postgres server.

```
# Create your func.yaml file
fn init <YOUR_DOCKERHUB_USERNAME>/func-postgres
# Build the function
fn build
# Test it
./test.sh
# Push it to Docker Hub
fn push
# Create routes to this function on IronFunctions
fn apps create <YOUR_APP> --config SERVER=<POSTGRES>
fn routes create --config TABLE=<TABLE_NAME> --config COMMAND=INSERT <YOUR_APP> /<TABLE_NAME>/insert
fn routes create --config TABLE=<TABLE_NAME> --config COMMAND=SELECT <YOUR_APP> /<TABLE_NAME>/select
```

Now you can call your function on IronFunctions:

```
echo <JSON_RECORD> | fn call /<YOUR_APP>/<TABLE_NAME>/insert
echo <JSON_QUERY> | fn call /<YOUR_APP>/<TABLE_NAME>/select
```