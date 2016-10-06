require 'json'

name = "World"

payload = STDIN.read 

STDERR.puts 'payload: ' + payload.inspect
if payload != ""
    payload = JSON.parse(payload)
    name = payload['name']
end

puts "Hello #{name}!"
