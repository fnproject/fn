require 'json'
require 'uri'

puts "Running checker..."

payload = STDIN.read
puts "payload #{payload}"
p ENV
if payload != ""
  payload = JSON.parse(payload)
  
  # payload contains checks
  if payload["env_vars"] 
    payload["env_vars"].each do |k,v|
      if ENV[k] != v 
        raise "Env var #{k} does not match"
      end
    end 
  end
  puts "all good"
end

# Also check for expected env vars: https://gitlab.oracledx.com/odx/functions/blob/master/docs/writing.md#inputs
e = ENV["FN_REQUEST_URL"]
puts e
uri = URI.parse(e)
if !uri.scheme.start_with?('http') 
  raise "invalid REQUEST_URL, does not start with http"
end
e = ENV["FN_METHOD"]
if !(e == "GET" || e == "POST" || e == "DELETE" || e == "PATCH" || e == "PUT")
  raise "Invalid METHOD: #{e}"
end
e = ENV["FN_APP_NAME"]
if e == nil || e == ''
  raise "No APP_NAME found"
end
e = ENV["FN_ROUTE"]
if e == nil || e == ''
  raise "No ROUTE found"
end
