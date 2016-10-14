require 'iron_worker'

# p IronWorker.payload
# puts "#{IronWorker.payload["input"]}"

payload = JSON.parse(ENV['PAYLOAD'])
puts payload['input']
