require 'rest'

rest = Rest::Client.new
rest.logger.level = Logger::DEBUG
response = rest.post("http://localhost:8080/", 
	headers: {"Iron-Router"=>"YES!"},
	body: {"host"=>"localhost", "dest"=>"localhost:8082"})
puts "body:"
puts response.body

