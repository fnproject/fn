require 'json'

name = "World"

# payload = STDIN.read 
# or using env vars: ENV['PAYLOAD']
payload = ENV['PAYLOAD']

STDERR.puts 'payload: ' + payload.inspect
if payload != ""
    payload = JSON.parse(payload)
    name = payload['name']
end

puts "Hello #{name}!"
