require 'json'

payload = STDIN.read
if payload != ""
  payload = JSON.parse(payload)
  
  # payload contains checks
  if payload["sleep"] 
    i = payload['sleep'].to_i
    puts "Sleeping for #{i} seconds..."
    sleep i
    puts "I'm awake!"
  end
end
