## Quick Example for a Ruby Function (4 minutes)

This example will show you how to test and deploy a Ruby function to IronFunctions.

```sh
# create your func.yaml file
fn init <YOUR_DOCKERHUB_USERNAME>/hello
# install dependencies, we need the json gem to run this
docker run --rm -it -v ${pwd}:/worker -w /worker iron/ruby:dev bundle install --standalone --clean
# build the function
fn build
# test it
cat hello.payload.json | fn run
# push it to Docker Hub
fn push
# Create a route to this function on IronFunctions
fn routes create myapp /hello
```

Now surf to: http://localhost:8080/r/myapp/hello

## Dependencies

Create a [Gemfile](http://bundler.io/gemfile.html) file in your function directory, then run:

```sh
docker run --rm -it -v ${pwd}:/worker -w /worker iron/ruby:dev bundle install --standalone --clean
```

Ruby doesn't pick up the gems automatically, so you'll have to add this to the top of your `func.rb` file:

```ruby
require_relative 'bundle/bundler/setup'
```

Open `func.rb` to see it in action.

To update dependencies:

```sh
docker run --rm -it -v ${pwd}:/worker -w /worker iron/ruby:dev bundle update
# then install again to vendor them
docker run --rm -it -v ${pwd}:/worker -w /worker iron/ruby:dev bundle update
```
