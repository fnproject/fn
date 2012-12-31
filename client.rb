require 'rest'

rest = Rest::Client.new
rest.logger.level = Logger::DEBUG
response = rest.get(
#"http://localhost:8080/"
 "http://routertest.irondns.info/"
)

puts "body:"
puts response.body
