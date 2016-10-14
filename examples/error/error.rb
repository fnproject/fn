require 'iron_worker'

if ENV['PAYLOAD'] && ENV['PAYLOAD'] != ""
    payload = JSON.parse(ENV['PAYLOAD'])
    puts payload['input']
end

raise "Something went terribly wrong!"
