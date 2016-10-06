require 'json'

name = "World"

payload = ENV['PAYLOAD']
if payload == nil || payload == "" 
    payload = STDIN.read
end

STDERR.puts 'payload: ' + payload.inspect
if payload != ""
    payload = JSON.parse(payload)
    name = payload['name']
end

puts "Hello #{name}!"
