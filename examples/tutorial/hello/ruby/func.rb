require 'json'

name = "World"

payload = STDIN.read
if payload != ""
    payload = JSON.parse(payload)
    name = payload['name']
end

puts "Hello #{name}!"

STDERR.puts "---> STDERR goes to server logs"
