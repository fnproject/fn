require 'rest'

rest = Rest::Client.new
rest.logger.level = Logger::DEBUG
response = rest.get("http://localhost:8080/") # "http://www.github.com")

puts "body:"
puts response.body
