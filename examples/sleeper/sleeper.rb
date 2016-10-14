require 'json'

payload = JSON.parse(ENV['PAYLOAD'])

i = payload['sleep'].to_i
puts "Sleeping for #{i} seconds..."
sleep i
puts "I'm awake!"
