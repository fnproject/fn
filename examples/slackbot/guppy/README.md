## Quick Example for a SlackBot command in Ruby

This example will show you how to test and deploy a SlackBot command to IronFunctions.

```sh
# create your func.yaml file
fn init <YOUR_DOCKERHUB_USERNAME>/guppy
# install dependencies, we need the json gem to run this
docker run --rm -v "$PWD":/worker -w /worker iron/ruby:dev bundle install --standalone --clean
# build the function
fn build
# test it
cat slack.payload | fn run
# push it to Docker Hub
fn push
# Create a route to this function on IronFunctions
fn routes create slackbot /guppy
# Change the route response header content-type to application/json
curl -X PUT http://127.0.0.1:8080/v1/apps/slackbot/routes/guppy -d '{ "route": { "headers": { "Content-type": ["application/json"] } } }'
```

## Create a Slash Command integration in Slack

In Slack, go to Integrations, find Slash Commands, click Add, type in / as the command then click Add again. On the next page, take the IronFunctions route URL and paste it into the URL field then click Save Integration.

If running in localhost, use [ngrok](https://github.com/inconshreveable/ngrok).

# Try it out!

In slack, type /<COMMAND> [options] and you'll see the magic!


