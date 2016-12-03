require 'json'

payload = STDIN.read
if payload != ""
  payload = JSON.parse(payload)
  
  # payload contains checks
  if payload["sleep"] 
    i = payload['sleep'].to_i
    STDERR.puts "Sleeping for #{i} seconds..."
    sleep i
    puts "I'm awake!" # sending this to stdout for sync response
  end
else 
  puts "ERROR: please pass in a sleep value: {\"sleep\": 5}"
end
