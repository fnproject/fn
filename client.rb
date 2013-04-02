require 'rest'

Rest.logger.level = Logger::DEBUG
rest = Rest::Client.new

base_url = "http://routertest.irondns.info/"
#"http://localhost:8080/"

response = rest.get(base_url)
puts "body:"
puts response.body

# test post
rest.post(base_url, :form_data=>{:x=>1, :y=>"a"})
