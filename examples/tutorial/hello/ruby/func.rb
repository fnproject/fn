require_relative 'bundle/bundler/setup'
require 'json'

name = "World"

payload = STDIN.read
if payload != ""
    payload = JSON.parse(payload)
    name = payload['name']
end

puts "Hello #{name} from Ruby!"
