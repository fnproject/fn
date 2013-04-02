require 'rest'

Rest.logger.level = Logger::DEBUG
rest = Rest::Client.new

base_url = "http://routertest.irondns.info/"
#"http://localhost:8080/"

response = rest.get(base_url)
puts "body:"
puts response.body
puts "\n\n"


# test post
begin
r = rest.post("#{base_url}somepost", :form_data=>{:x=>1, :y=>"a"})
rescue Rest::HttpError => ex
    p ex
end
p r
p r.body
