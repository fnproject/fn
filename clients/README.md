`build.rb` will generate all clients for all Swagger supported languages.

## Building and Deploying Clients

### First Time

If this is your first time building the clients, you'll need to do the following:

```sh
bundle install
```

### Every Time

Everytime the API spec is updated, be sure to bump the version number in `swagger.yml`, then run:

```sh
ruby build.rb
```

Boom. That's it.

## Troubleshooting

Sometimes this will fail due to github caching or something and versions will be off. Just bump version and retry.
