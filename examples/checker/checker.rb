require 'json'

payload = JSON.parse(ENV['PAYLOAD'])

# payload contains checks
if payload["env_vars"] 
  payload["env_vars"].each do |k,v|
    if ENV[k] != v 
      raise "Env var #{k} does not match"
    end
  end 
end
puts "all good"
