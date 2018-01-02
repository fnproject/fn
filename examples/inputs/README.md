# Tutorial 1: Go Function w/ Input (3 minutes)

Example for using required and optional environment variables defined in the func.yaml file.

Try running the following:

```sh
fn run
# > ERROR: required env var SECRET_1 not found, please set either set it in your environment or pass in `-e SECRET_1=X` flag.
SECRET_1=YOOO fn run
# > info: optional env var SECRET_2 not found.
# > {"SECRET_1":"YOOO","SECRET_2":"","message":"Hello World"}
SECRET_1=YOOO SECRET_2=DAWG fn run
# > {"SECRET_1":"YOOO","SECRET_2":"DAWG","message":"Hello World"}
```
